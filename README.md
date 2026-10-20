# Port Sweeper

A port scanner written in Golang.

## Features

- Single port 
- Multi-concurrent port checking
- Timing abilities
- Interrupt support


## Usage

```bash
git clone git@github.com:pierrecdevs/go-portsweeper.git
cd go-portsweeper
go build .../.go
./main -host scanme.nmap.org -ports 1-1024 # multi port
./main -host scanme.nmap.org -port 80 # single port
./main -host scanme.nmap.org -port 1-65534 -workers 100 # multi port, worker tuning
```
