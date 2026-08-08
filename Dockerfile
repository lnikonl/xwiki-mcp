ARG REGISTRY=latest
FROM $REGISTRY/golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /xwiki-mcp .

FROM scratch
COPY --from=build /xwiki-mcp /xwiki-mcp
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8080
ENTRYPOINT ["/xwiki-mcp"]
