# UDPRainbowBridge

UDPRainbowBridge is a UDP-based networking project written in Go. 


## Build Instructions

### Windows (Using build.bat)
Open a command prompt and run:

```bat
build.bat
```

The script builds binaries for multiple platforms and stores them in the `out` directory.


## Usage

### Server Mode

Run the server mode by specifying the `-s` flag along with necessary addresses. Example:

```sh
go run main.go -s -r 192.168.2.3:8080 -l 0.0.0.0:9000
```

- **-s**: Run in server mode.
- **-r**: Remote address (only one address for server mode).
- **-l**: Listening addresses (multiple allowed for server mode).

### Client Mode

Run the client mode by specifying the `-c` flag and providing the sending and listening addresses. Example:

```sh
go run main.go -c -r 192.168.2.3:8080;192.168.2.110:8080 -l 0.0.0.0:9000 -send 192.168.100.1:0;192.168.99.1:0
```

- **-c**: Run in client mode.
- **-r**: Remote addresses (multiple are allowed separated by semicolons).
- **-l**: Listening address (single allowed for client mode).
- **-send**: Local sending addresses (use port 0 for auto-selection).


