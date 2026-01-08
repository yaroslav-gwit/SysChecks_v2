# Use Ubuntu 18.04 LTS for maximum compatibility with older systems
# Ubuntu 18.04 has glibc 2.27 and is supported until April 2028
FROM ubuntu:18.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    wget \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install the latest Go version
# We'll use Go 1.21+ for modern features while maintaining compatibility
ENV GO_VERSION=1.21.5
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOROOT/bin:$GOPATH/bin:$PATH

RUN wget -O go.tar.gz "https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    && tar -C /usr/local -xzf go.tar.gz \
    && rm go.tar.gz

# Create workspace
WORKDIR /app

# Copy go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version information
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# Build the application
# CGO_ENABLED=0 creates a fully static binary
# -ldflags='-w -s' strips debug information to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static' -X 'syschecks/cmd.Version=${VERSION}' -X 'syschecks/cmd.GitCommit=${GIT_COMMIT}' -X 'syschecks/cmd.BuildDate=${BUILD_DATE}'" \
    -o bin/syschecks \
    .

# Create a minimal final image
FROM scratch

# Copy the binary from builder
COPY --from=0 /app/bin/syschecks /syschecks

# Copy CA certificates for HTTPS requests
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Set the entrypoint
ENTRYPOINT ["/syschecks"]
