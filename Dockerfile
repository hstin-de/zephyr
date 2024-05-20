# Stage 1: Build stage
FROM golang:1.22.3 AS builder

# Install dependencies
RUN apt update && apt install -y wget bzip2 build-essential libeccodes-dev \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app 

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the project
RUN make build-prod

# Stage 2: Final stage
FROM ghcr.io/hstin-de/cdo

# Install only necessary runtime dependencies
RUN apt update && apt install -y libeccodes-dev \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/build/zephyr /app/zephyr

# Ensure the binary is executable
RUN chmod +x ./zephyr

# Set the entry point to the binary
ENTRYPOINT ["./zephyr"]
