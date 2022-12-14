package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	gRPC "github.com/LiZi-77/PCPP-Assignment4/grpc"
	"google.golang.org/grpc"
)

// problem: no go mod
// go mod init Assignment4
// go get google.golang.org/grpc



var port = flag.Int("port", 5000, "port") //port for the node default 5000

const INITPORT int32 = 8000

type STATE int32 //state of the node

const (
	RELEASED STATE = iota
	HELD
	REQUESTED
)

type peer struct {
	gRPC.UnimplementedPingServer
	id            	int32
	clients       	map[int32]gRPC.PingClient
	ctx           	context.Context
	state			STATE
	requests		int32 // the number of requests sent by this node
	queue    		[]int32
}

func main() {
	f := setLog() //uncomment this line to log to a log.txt file instead of the console
	defer f.Close()

	flag.Parse() //go run client.go -port to set the port of this peer

	ctx_, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ownPort = int32(*port)

	//create a node for this proccess
	p := &peer{
		id:       ownPort,
		clients:  make(map[int32]gRPC.RingClient),
		ctx:      ctx_,
		state:    RELEASED,	// initial state is RELEASED
		requests: 0,
	}

	//creates listener on port 
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v\n", err)
	}

	log.Printf("Node created on port: %d\n", p.id)
	fmt.Printf("Node created on port %v\n", p.id)

	grpcServer := grpc.NewServer()
	gRPC.RegisterRingServer(grpcServer, p)

	//serve on the listener
	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v\n", err)
		}
	}()
	
	//for loop to dial all other nodes in the network, if this loop is increased the number of nodes in the network is aswell
	for i := 0; i < 3; i++ {
		nodePort := int32(5000 + i)

		if nodePort == p.id {
			continue
		}

		var conn *grpc.ClientConn
		log.Printf("Trying to dial: %v\n", nodePort)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", nodePort), grpc.WithInsecure(), grpc.WithBlock())

		if err != nil {
			log.Fatalf("Could not connect: %v\n", err)
		}

		defer conn.Close()
		log.Printf("Succes connecting to: %v\n", nodePort)
		client := gRPC.NewRingClient(conn)
		p.clients[nodePort] = client

		log.Printf("peer %v is connected to peer %v \n", p.id, nodePort)
		fmt.Printf("peer %v is connected to peer %v \n", p.id, nodePort)
	}

	p.systemStart()
	// read the input from current node 
}

func (p *peer) systemStart() {
	println("3 peers system starts now.")

	scanner := bufio.NewScanner(os.Stdin)
	println("Welcome to the 3 peer system!\n Please input 1 if you want to do the critical section.")

	for {
		scanner.Scan()
		text := scanner.Text()

		if text == "1" {
			//sendRequest to all peers
			log.Println("%v want to get the critical section.", p.id)
			p.sendRequestToAllPeers()
		} else {
			println("Please input 1 if you want to do the critical section. \n Other inputs are illegal.")
		}
	}
}

func (p *peer) sendRequestToAllPeers() {
	// [peerId] sends requests to all peers to get permission
	p.state = REQUESTED 	// change p's state to REQUESTED

	request := gRPC.Request{
		Id: p.id,	// this field clarify which peer sent this request
	}

	log.Printf("%v is sending request to all other nodes. Missing %d replies.\n", p.id, p.requests)
	fmt.Println("Sending request to all other nodes")

	//send request to all clients in the clients map
	for id, client := range p.clients {
		_, err := client.AccessRequest(p.ctx, request)

		if err != nil {
			log.Printf("Can't send access request to peer: %v, error: %v\n", id, err)
		}
	}
}

func (p *peer)AccessRequest(ctx context.Context, req *gRPC.Request) (*gRPC.Ack, error) {
	log.Printf("peer %v recieved a request from peer %v.\n", p.id, req.Id)
	fmt.Println("Recieved a request")

	// We order the hierachy by the peer id which means 5000 has a higher rannk
	if p.state == HELD || (p.state == REQUESTED && (p.id < req.Id)) {
		log.Printf("%v is queueing a request from %v\n", p.id, req.Id)
		fmt.Println("Queueing request")
		p.queue = append(p.queue, req.Id)
	} else {
		if p.state == REQUESTED {
			p.requests++
			p.clients[req.Id].RequestAccess(ctx, &gRPC.Request{Id: p.id})
		}
		log.Printf("%v is sending a reply to %v\n", p.id, req.Id)
		fmt.Println("Sending reply")
		p.clients[req.Id].Reply(ctx, &gRPC.Reply{})
	}
	reply := &gRPC.Ack{}
	return reply, nil
}

// sets the logger to use a log.txt file instead of the console
func setLog() *os.File {

	// This connects to the log file/changes the output of the log informaiton to the log.txt file.
	f, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(f)
	return f
}

