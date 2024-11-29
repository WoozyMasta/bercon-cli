# BattlEye RCon Protocol Specification v2

> Copyright (C) 2011 by BattlEye Innovations

[Original document](https://www.battleye.com/downloads/BERConProtocol.txt)

## General

BE RCon uses the game server's network interface, i.e. its UDP game port.  
Due to the unreliable nature of UDP the client has to take care of potential packet loss.

The protocol specifies the following 7-byte header in all BE RCon packets, incoming and outgoing:

```txt
'B'(0x42) | 'E'(0x45) | 4-byte CRC32 checksum of the subsequent bytes | 0xFF
```

The subsequent bytes (payload) describe the actual RCon packet (see below).

## Packet types

There are 3 different types of packets:

### Login packet

This is the first packet being sent to the server.
A successful login is required to send actual commands to the server.

The format is as follows:

```txt
0x00 | password (ASCII string without null-terminator)
```

The server's BE RCon, if enabled, acknowledges with the following packet:

```txt
0x00 | (0x01 (successfully logged in) OR 0x00 (failed))
```

If the server doesn't respond, BE RCon is not enabled (no password specified).

### Command packet

After a client logged in successfully, it may send BE Server commands (and possibly game server commands) to the server.

The format is as follows:

```txt
0x01 | 1-byte sequence number (starting at 0) | command (ASCII string without null-terminator)
```

The server's BE RCon acknowledges with the following packet:

```txt
0x01 | received 1-byte sequence number | (possible header and/or response (ASCII string without null-terminator) OR nothing)
```

The following header exists only if the server responds with multiple packets due to packet size limitations. The header is present in each of those packets.

```txt
0x00 | number of packets for this response | 0-based index of the current packet
```

An empty 2-byte command packet (without actual command string) has to be sent every 45 seconds (or less) to keep the connection/login alive, if there are no other command packets being sent.  
If there are no command packets coming from the client for more than 45 seconds, it will be removed from BE RCon's list of authenticated clients and will no longer be able to issue any commands.

### Server message packet

When the BE Server prints messages to the server console, it also sends them to all connected RCon clients.

The format is as follows:

```txt
0x02 | 1-byte sequence number (starting at 0) | server message (ASCII string without null-terminator)
```

The client has to acknowledge with the following packet:

```txt
0x02 | received 1-byte sequence number
```

The server's BE RCon tries to send a server message packet 5 times for 10 seconds. If the client fails to acknowledge the packet within this time, it will be removed from BE RCon's list of authenticated clients and will no longer be able to issue any commands.
