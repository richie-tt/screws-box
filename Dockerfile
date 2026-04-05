FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /screws-box ./cmd/screwsbox

FROM scratch
COPY --from=build /screws-box /screws-box
EXPOSE 8080
ENTRYPOINT ["/screws-box"]
