FROM ghcr.io/hstin-de/cdo

RUN apt update && apt install -y wget bzip2 build-essential libeccodes-dev file

# Download golang
RUN wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz

# Install golang
RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz

# Set PATH
ENV PATH=$PATH:/usr/local/go/bin

WORKDIR /app 

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the project
RUN make build-prod

# Ensure the binary is executable
RUN chmod +x ./build/zephyr

# Debugging: Log the existence and type of the binary
RUN ls -l ./build/zephyr
RUN file ./build/zephyr

# Set the entry point to the binary
ENTRYPOINT ["./build/zephyr"]
