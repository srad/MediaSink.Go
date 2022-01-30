# Alpine is chosen for its small footprint
# compared to Ubuntu
FROM golang:latest

WORKDIR /app

# Download necessary Go modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# ... the rest of the Dockerfile is ...
# ...   omitted from this example   ...