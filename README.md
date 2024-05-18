# zephyr: A High-Performance Weather API Server

`zephyr` is a high-performance weather API server written in Go, using the custom binary format [ndfile](https://github.com/hstin-de/ndfile) for data storage. It integrates an HTTP server, gRPC server, and downloader within a single binary.

## Features

- **HTTP Server:** Serve weather data over HTTP on customizable ports.
- **gRPC Server:** Provide weather data over gRPC for high-performance needs.
- **Data Downloader:** Automatically fetch the latest weather data from remote sources. (Requires cdo 1.9.10)
- **Customizable Parameters:** Select specific weather parameters to fetch and serve.

## Prequisites:
- go 1.22.3
- [cdo 1.9.10 with netcdf and grib2 support](https://gist.github.com/jeffbyrnes/e56d294c216fbd30fd2fd32e576db81c) (for downloading data)

## Getting Started

### Building `zephyr`

Build `zephyr` for production with:

```bash
make build-prod
```

That will create a binary named `zephyr` in the `build/` directory.

### Running `zephyr`

To see available commands and options, run:

```bash
./build/zephyr --help
```

### Configuration Options

- `--http`: Start the HTTP server (default: false)
- `--grpc`: Start the gRPC server (default: false)
- `--download, --dl`: Download the newest weather data (default: false)
- `--http-port value`: HTTP server port (default: "8081")
- `--grpc-port value`: gRPC server port (default: "50051")
- `--params value, -p value [ --params value, -p value ]`: Parameters to fetch (default: various weather parameters)
- `--help, -h`: Show help

## License

`zephyr` is licensed under the Apache-2.0 License. See the [LICENSE](LICENSE) file for more details.

Enjoy using `zephyr`!