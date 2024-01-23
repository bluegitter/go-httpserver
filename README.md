# Go HTTP Server

## Introduction

This is a basic HTTP server written in Go. It supports serving static files, logging HTTP requests, and rotating log files based on the date.

## Features

- Static File Serving: Acts as a basic file server to serve static content.
- Logging: Records all HTTP requests including IP address, request method, URL, status code, processing time, and response size.
- Log File Rotation: Supports log file rotation based on the date, automatically moving logs to new files and continuing logging across days.

## Installation

To install and run this HTTP server, ensure you have Go installed on your system.

1. Clone the repository:

```
git clone https://github.com/bluegitter/go-httpserver.git
```

2. Build the project:

```
cd go-httpserver
go mod tidy
GOOS=linux GOARCH=amd64 go build -o server server.go
```

This will create an executable file named `server` in the current directory.

## Usage

To start the server, use the following command:

```
./server -p <port>
```

Where `<port>` is the port number you want the server to listen on. For example, `./server -p 8080` will start the server on port 8080.

## Contributing

Contributions of any kind are welcome, including feature proposals, code submissions, bug reports, and documentation updates.

## License

This project is licensed under the [MIT License](LICENSE).
