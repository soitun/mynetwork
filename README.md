# Mynetwork

A Lightweight VPN Built on top of IPFS & Libp2p for Truly Distributed Networks. 

[Documentation](https://docs.mynetwork.99400.cn/)

## Table of Contents
- [A Bit of Backstory](#a-bit-of-backstory)
- [Use Cases](#use-cases)
  - [A Digital Nomad](#a-digital-nomad)
  - [A Privacy Advocate](#a-privacy-advocate)
- [Usage](#usage)
  - [Commands](#commands)
- [Tutorial](#tutorial)
- [Hacking](#hacking)

## A Bit of Backstory
[Libp2p](https://libp2p.io) is a networking library created by [Protocol Labs](https://protocol.ai) that allows nodes to discover each other using a Distributed Hash Table. Paired with [NAT hole punching](https://en.wikipedia.org/wiki/Hole_punching_(networking)) this allows Mynetwork to create a direct encrypted tunnel between two nodes even if they're both behind firewalls.

**Moreover! Each node doesn't even need to know the other's ip address prior to starting up the connection.** This makes Mynetwork perfect for devices that frequently migrate between locations but still require a constant virtual ip address.

## Use Cases:
##### A Digital Nomad
I use this system when travelling, if I'm staying in a rental or hotel and want to try something out on a Raspberry Pi I can plug the Pi into the location's router or ethernet port and then just ssh into the system using the same-old internal Mynetwork ip address without having to worry about their NAT or local firewall. Furthermore, if I'm connected to the Virtual Mynetwork Network I can ssh into my machines at home without requiring me to set up any sort of port forwarding.

##### A Privacy Advocate
Honestly, I even use this system when I'm at home and could connect directly to my local infrastructure. Using Mynetwork however, I don't have to trust the security of my local network and Mynetwork will intelligently connect to my machines using their local ip addresses for maximum speed.

If anyone else has some use cases please add them! Pull requests welcome!

| :exclamation: | Mynetwork is still a very new project. Although we've tested the code locally for security, it hasn't been audited by a third party yet. We probably wouldn't trust it yet in high security environments. | 
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |

## Getting Started

## Usage

### Commands

| Command             |  Alias  | Description                                                                |
| ------------------- | ------- | -------------------------------------------------------------------------- |
| `help`              | `?`     | Get help with a specific subcommand.                                       |
| `init`              | `i`     | Initialize an interface's configuration.                                   |
| `up`                | `up`    | Create and bring up a Mynetwork interface                                  |
| `status`            | `s`     | Inspect the status of a Mynetwork daemon                                   |
| `peers`             |         | List connected LibP2P peers                                                |
| `route`             | `r`     | Inspect and modify the route table                                         |

### Global Flags
| Flag                |  Alias  | Description                                                                |
| ------------------- | ------- | -------------------------------------------------------------------------- |
| `--config`          | `-c`    | Specify the path to a Mynetwork config for an interface.                   |
| `--interface`       | `-i`    | The Mynetwork interface to operate on.                                     |


## Tutorial

### Initializing an Interface

The first thing we'll want to do once we've got Mynetwork installed is
initialize the configuration for an interface. The default interface name is `mynetwork`.
In this example we'll call the interface on our local machine `ms0` (for mynetwork 0) and `ms1` on our remote server but yours could be anything you'd like.

(Note: if you're using a Mac you'll have to use the interface name `utun[0-9]`. Check which interfaces are already in use by running `ip a` once you've got `iproute2mac` installed.)

(Note: if you're using Windows you'll have to use the interface name as seen in Control Panel. IP address will be set automatically only if you run Mynetwork as Administrator.)

###### Local Machine
```shell-session
$ sudo mynetwork init -i ms0
```

###### Remote Machine
```shell-session
$ sudo mynetwork init -i ms1
```

### Add Each Machine As A Peer Of The Other

Now that we've got a set of configurations we'll want to
tell the machines about each other. By default Mynetwork will
put the interface configurations in `interface-name.json`.
You can also create the config file elsewhere by specifying a custom config path.

```shell-session
$ mynetwork init -c mynetwork-config-ms0.json
```

So for our example we'll run

###### Local Machine
```shell-session
$ sudo $EDITOR ./ms0.json
```

and

###### Remote Machine
```shell-session
$ sudo $EDITOR ./ms1.json
```

### Update Peer Configs

Now in each config we'll add the other machine's ID as a peer.
Mynetwork will print a config snippet like this one and instruct you to add it to your other peers:
```json
{
  "name": "ms1",
  "id": "12D3KExamplePeer1"
}
```

Update

```json
{
  "peers": [],
  "privateKey": "z23ExamplePrivateKey"
}
```
to 
```json
{
  "peers": [
    {
      "name": "hostname1",
      "id": "12D3KExamplePeer1"
    }
  ],
  "privateKey": "z23ExamplePrivateKey"
}
```

Previously, it was necessary to manually configure IP addresses for all peers. Now, an IP address from the 100.64.0.0/16 range is automatically allocated to each peer based on its peer ID.

You can specify additional routes for each peer as well:

```json
{
  "peers": [
    {
      "name": "hostname1",
      "id": "12D3KExamplePeer1"
      "routes": [
        { "net": "10.1.1.1/32" },
        { "net": "10.1.2.0/24" }
      ]
    }
  ],
  "privateKey": "z23ExamplePrivateKey"
}
```

### Starting Up the Interfaces!
Now that we've got our configs all sorted we can start up the two interfaces!

###### Local Machine
```shell-session
$ sudo mynetwork up -i ms0
```

and

###### Remote Machine
```shell-session
$ sudo mynetwork up -i ms1  
```

After a few seconds you should see a the network finish setting up
and find your other machine. We can now test the connection by
pinging back and forth across the network.

###### Local Machine
```shell-session
$ ping 100.64.90.181
```

We can get some more information about the status of the network as well.

###### Local Machine
```shell-session
$ sudo mynetwork status -i hs0
PeerID: 12D3KExamplePeer1
Swarm peers: 7
Connected VPN nodes: 1/1
    @hostname2 (365.516µs) /ip4/.../udp/8001/quic-v1/p2p/12D3KExamplePeer2
Addresses:
    /ip4/.../tcp/8001
    (...)
```

### Stopping the Interface and Cleaning Up
Now to stop the interface and clean up the system, simply kill the proceses (for example, by pressing Ctrl+C where you started it).

## Hacking

If you want to hack on Mynetwork, check out the [development docs](https://docs.mynetwork.99400.cn/Development.html) for a quick introduction.

## Disclaimer & Copyright

Wireguard is a registered trademark of Max Headroom.

## License

Copyright 2025- Stone Fan <soitun.fan@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
