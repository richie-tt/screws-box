package store

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errInjected is the canonical error returned by the fault-injecting driver.
var errInjected = errors.New("injected driver fault")

// faultPlan controls when the wrapping driver should fail. Each counter,
// when > 0, decrements on every matching call; the call that drops it to
// zero returns errInjected. A counter of 0 means "do not inject".
type faultPlan struct {
	mu              sync.Mutex
	failBeginAfterN int // number of Begin calls to allow before failing the next one
	failExecAfterN  int // number of Exec/Stmt.Exec calls to allow before failing the next one
	failQueryAfterN int // number of QueryRow/Query calls to allow before failing the next one
	failCommit      bool
	// failExecOn fails the first Exec whose prepared SQL contains this substring.
	// One-shot: cleared after firing. Empty string disables.
	failExecOn string
}

func (p *faultPlan) tripBegin() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failBeginAfterN < 0 {
		return false
	}
	if p.failBeginAfterN == 0 {
		p.failBeginAfterN = -1 // one-shot
		return true
	}
	p.failBeginAfterN--
	return false
}

// tripExecMatching is a one-shot trigger that fires the first time an Exec
// is invoked with a SQL string containing failExecOn.
func (p *faultPlan) tripExecMatching(sqlText string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failExecOn == "" {
		return false
	}
	if !contains(sqlText, p.failExecOn) {
		return false
	}
	p.failExecOn = "" // one-shot
	return true
}

// contains is a tiny strings.Contains shim kept inline to avoid an extra import.
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func (p *faultPlan) tripExec() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failExecAfterN < 0 {
		return false
	}
	if p.failExecAfterN == 0 {
		p.failExecAfterN = -1
		return true
	}
	p.failExecAfterN--
	return false
}

func (p *faultPlan) tripQuery() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failQueryAfterN < 0 {
		return false
	}
	if p.failQueryAfterN == 0 {
		p.failQueryAfterN = -1
		return true
	}
	p.failQueryAfterN--
	return false
}

// noPlan disables all fault injection; used when no plan is registered.
var noPlan = &faultPlan{failBeginAfterN: -1, failExecAfterN: -1, failQueryAfterN: -1}

// activePlan is the plan consulted by the wrapping driver. Set via withPlan.
var activePlan = noPlan

func withPlan(t *testing.T, p *faultPlan) {
	t.Helper()
	prev := activePlan
	activePlan = p
	t.Cleanup(func() { activePlan = prev })
}

// faultDriver wraps modernc.org/sqlite and consults activePlan on every call.
type faultDriver struct{ inner driver.Driver }

func (d *faultDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &faultConn{inner: c}, nil
}

type faultConn struct{ inner driver.Conn }

func (c *faultConn) Prepare(query string) (driver.Stmt, error) {
	s, err := c.inner.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &faultStmt{inner: s, sql: query, isQuery: looksLikeQuery(query)}, nil
}
func (c *faultConn) Close() error { return c.inner.Close() }
func (c *faultConn) Begin() (driver.Tx, error) {
	if activePlan.tripBegin() {
		return nil, errInjected
	}
	tx, err := c.inner.Begin() //nolint:staticcheck // legacy interface required
	if err != nil {
		return nil, err
	}
	return &faultTx{inner: tx}, nil
}

type faultTx struct{ inner driver.Tx }

func (t *faultTx) Commit() error {
	if activePlan.failCommit {
		activePlan.failCommit = false
		return errInjected
	}
	return t.inner.Commit()
}
func (t *faultTx) Rollback() error { return t.inner.Rollback() }

type faultStmt struct {
	inner   driver.Stmt
	sql     string
	isQuery bool
}

func (s *faultStmt) Close() error  { return s.inner.Close() }
func (s *faultStmt) NumInput() int { return s.inner.NumInput() }
func (s *faultStmt) Exec(args []driver.Value) (driver.Result, error) { //nolint:staticcheck // legacy interface required
	if activePlan.tripExecMatching(s.sql) {
		return nil, errInjected
	}
	if activePlan.tripExec() {
		return nil, errInjected
	}
	return s.inner.Exec(args) //nolint:staticcheck // legacy interface required
}

func (s *faultStmt) Query(args []driver.Value) (driver.Rows, error) { //nolint:staticcheck // legacy interface required
	if s.isQuery && activePlan.tripQuery() {
		return nil, errInjected
	}
	return s.inner.Query(args) //nolint:staticcheck // legacy interface required
}

func looksLikeQuery(sqlText string) bool {
	for i := range len(sqlText) {
		c := sqlText[i]
		if c == ' ' || c == '\t' || c == '\n' {
			continue
		}
		// SELECT / WITH / EXPLAIN are read paths; everything else we treat as exec.
		return c == 's' || c == 'S' || c == 'w' || c == 'W' || c == 'e' || c == 'E'
	}
	return false
}

var registerDriverOnce sync.Once

func registerFaultDriver() {
	registerDriverOnce.Do(func() {
		// Get the modernc.org/sqlite driver instance via a temporary sql.DB.
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			panic(fmt.Sprintf("registerFaultDriver: open sqlite: %v", err))
		}
		inner := db.Driver()
		_ = db.Close()
		sql.Register("sqlite-faulty", &faultDriver{inner: inner})
	})
}

// withFaultDriver swaps sqlDriverName to the wrapping driver for the duration
// of the test, registering it on first use.
func withFaultDriver(t *testing.T) {
	t.Helper()
	registerFaultDriver()
	prev := sqlDriverName
	sqlDriverName = "sqlite-faulty"
	t.Cleanup(func() { sqlDriverName = prev })
}

// --- Tests ----------------------------------------------------------------

// sql.Open returns "sql: unknown driver" when the driver isn't registered.
// This hits the sql.Open error branch in Store.Open (line 172-174).
func TestOpenUnknownDriver(t *testing.T) {
	prev := sqlDriverName
	sqlDriverName = "definitely-not-a-real-driver"
	t.Cleanup(func() { sqlDriverName = prev })

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open database")
}

// createSchema's tx.Begin failure → "begin schema tx" branch.
func TestOpenCreateSchemaBeginFails(t *testing.T) {
	withFaultDriver(t)
	withPlan(t, &faultPlan{
		failBeginAfterN: 0, // fail the very first Begin (which is createSchema's)
		failExecAfterN:  -1,
		failQueryAfterN: -1,
	})

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create schema")
}

// createSchema's tx.Exec failure → "schema exec" branch.
func TestOpenCreateSchemaExecFails(t *testing.T) {
	withFaultDriver(t)
	withPlan(t, &faultPlan{
		failBeginAfterN: -1,
		failExecAfterN:  0, // first Exec call is the first DDL inside createSchema
		failQueryAfterN: -1,
	})

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create schema")
}

// createSchema's tx.Commit failure → "commit schema" branch.
func TestOpenCreateSchemaCommitFails(t *testing.T) {
	withFaultDriver(t)
	withPlan(t, &faultPlan{
		failBeginAfterN: -1,
		failExecAfterN:  -1,
		failQueryAfterN: -1,
		failCommit:      true,
	})

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create schema")
}

// seedDefaultShelf's count-query failure → "check shelf count" branch.
// Schema commit (1) succeeds; the next QueryRow inside seedDefaultShelf fails.
func TestOpenSeedCountQueryFails(t *testing.T) {
	withFaultDriver(t)
	withPlan(t, &faultPlan{
		failBeginAfterN: -1,
		failExecAfterN:  -1,
		failQueryAfterN: 0, // first SELECT is seed's COUNT query
	})

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed default shelf")
}

// seedDefaultShelf's tx.Begin failure → "begin seed tx" branch.
// We let createSchema's Begin (1) and migrations (no Begin) pass, then fail
// the seed Begin (2).
func TestOpenSeedBeginFails(t *testing.T) {
	withFaultDriver(t)
	withPlan(t, &faultPlan{
		failBeginAfterN: 1, // 1 Begin allowed (schema), next Begin (seed) fails
		failExecAfterN:  -1,
		failQueryAfterN: -1,
	})

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed default shelf")
}

// seedDefaultShelf's INSERT shelf failure → "insert shelf" branch.
// Schema and migrations Execs all succeed; the first Exec whose SQL contains
// "INSERT INTO shelf" fails. Pattern-based so test stays robust as
// migrations are added.
func TestOpenSeedInsertShelfFails(t *testing.T) {
	withFaultDriver(t)
	withPlan(t, &faultPlan{
		failBeginAfterN: -1,
		failExecAfterN:  -1,
		failQueryAfterN: -1,
		failExecOn:      "INSERT INTO shelf",
	})

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed default shelf")
}

// seedDefaultShelf's INSERT container failure → "insert container" branch.
// Shelf insert succeeds; first INSERT INTO container fails.
func TestOpenSeedInsertContainerFails(t *testing.T) {
	withFaultDriver(t)
	withPlan(t, &faultPlan{
		failBeginAfterN: -1,
		failExecAfterN:  -1,
		failQueryAfterN: -1,
		failExecOn:      "INSERT INTO container",
	})

	s := &Store{}
	err := s.Open(t.TempDir() + "/x.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed default shelf")
}
