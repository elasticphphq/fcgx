# FastCGI Protocol Notes

## Overview
FastCGI is an open extension to CGI that provides high performance for all Internet applications without the penalties of Web server APIs. It supports long-lived application processes (application servers) that can handle multiple requests.

## Protocol Basics

### Records
All data that flows over the transport connection is carried in FastCGI records. Records serve two purposes:

1. **Multiplexing**: Records multiplex the transport connection between several independent FastCGI requests, supporting applications that process concurrent requests using event-driven or multi-threaded programming techniques.

2. **Multiple Streams**: Records provide several independent data streams in each direction within a single request. For example, both stdout and stderr data pass over a single transport connection.

### Record Structure
A FastCGI record consists of a fixed-length prefix followed by variable content and padding bytes:

```c
typedef struct {
    unsigned char version;
    unsigned char type;
    unsigned char requestIdB1;
    unsigned char requestIdB0;
    unsigned char contentLengthB1;
    unsigned char contentLengthB0;
    unsigned char paddingLength;
    unsigned char reserved;
    unsigned char contentData[contentLength];
    unsigned char paddingData[paddingLength];
} FCGI_Record;
```

Components:
- **version**: Identifies the FastCGI protocol version (FCGI_VERSION_1)
- **type**: Identifies the record type (function the record performs)
- **requestId**: Identifies the FastCGI request to which the record belongs
- **contentLength**: Number of bytes in the contentData component
- **paddingLength**: Number of bytes in the paddingData component
- **contentData**: 0 to 65535 bytes of data, interpreted according to record type
- **paddingData**: 0 to 255 bytes of data, ignored

### Transport Connections
- Applications accept connections from web servers on a listening socket
- The protocol supports both Unix stream pipes (AF_UNIX) and TCP/IP (AF_INET)

## Record Types

### Management Record Types
- FCGI_GET_VALUES: Used to query variables
- FCGI_GET_VALUES_RESULT: Response to query
- FCGI_UNKNOWN_TYPE: Response to unrecognized record type

### Application Record Types
- FCGI_BEGIN_REQUEST: Starts a request
- FCGI_PARAMS: Contains name-value pairs from server to application
- FCGI_STDIN: Data from server to application
- FCGI_DATA: Additional data stream from server to application
- FCGI_STDOUT: Data from application to server
- FCGI_STDERR: Error output from application to server
- FCGI_ABORT_REQUEST: Aborts a request
- FCGI_END_REQUEST: Ends a request

## Roles
FastCGI applications can play different roles:
- **Responder**: Receives HTTP request info and generates HTTP response (like CGI)
- **Authorizer**: Receives HTTP request info and generates authorized/unauthorized decision
- **Filter**: Receives HTTP request info plus data stream, generates filtered data stream

## Implementation Considerations for Go Client

### Connection Handling
- Must support accepting connections on a listening socket
- Should handle both Unix sockets and TCP/IP connections
- Need to implement proper connection pooling

### Record Processing
- Must correctly parse and generate FastCGI records
- Need to handle multiplexing of multiple requests over a single connection
- Must support all required record types

### Stream Management
- Need to handle multiple data streams (STDIN, STDOUT, STDERR)
- Must properly manage request lifecycle (BEGIN_REQUEST to END_REQUEST)

### Context Support Requirements
- Must integrate with Go's context package for timeout and cancellation
- Should allow setting timeouts for connection establishment
- Should allow setting timeouts for request completion
- Must support cancellation of in-progress requests
- Should handle cleanup of resources when context is cancelled

### Error Handling
- Must properly handle protocol errors
- Should provide meaningful error messages
- Need to handle network errors gracefully
