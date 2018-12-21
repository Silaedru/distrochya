# Distrochya

Distributed chat application written in Go. Proof of concept rather than fully finished and polished product.

Depends on [gocui](https://github.com/jroimartin/gocui) for UI.

## How it works:
 * A node needs to start a new network - when it does so, it's automatically elected as its leader
 * When a new node connects, it becomes the new successor to the *known* node (which it used to join the network)
 * Virtual ring used for leader election is separate from virtual star used for chatting
 * When a node's successor is lost, it tries to connect to the old successor's successor first (if that fails, it sends ```closering``` request through previous node)
 * When a leader is lost, each node waits a random amount of time before starting a new election, except for the old leader's predecessor, which starts election immediately once it detects that the ring topology has been fixed

## Limitations:
 * Node IP is taken from the first non-loopback interface that has an IP address
 * Node IDs are based on said IPs (won't work through NAT etc.)
 * Current protocol doesn't allow sending newline character (```\n```) and semicolons must be handled with care

## Some remarks:
 * Connected nodes elect a leader node between themselves (when an existing leader is lost) that works as a chat server using Chang-Roberts algorithm
 * The entire system tries to keeps itself in a consistent state (each node has a successor, leader is elected)
 * The system is able to reliably handle a single node failure at a time
 * Chat functionality itself is rather basic
 * Synchronization is an incredible mess that works by the sheer force of will
 * Not the cleanest Go codebase there is (certainly not idiomatic)
