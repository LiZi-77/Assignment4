package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	GroupThree "github.com/LiZi-77/PCPP-Assignment4/grpc"
	"google.golang.org/grpc"
)

var port = flag.Int("port", 5000, "port") //port for the node default 5000

type STATE int32 //state struct to see wether the node has access, wants access or not.

const (
	RELEASED STATE = iota
	HELD
	REQUESTED
)

// node struct.
type node struct {
	GroupThree.UnimplementedGroup3Server
	requests int
	id       int32 //port
	clients  map[int32]GroupThree.Group3Client
	ctx      context.Context
	queue    []int32
	state    STATE
}

// request function to request access to the critical service
func (n *node) Request(ctx context.Context, req *GroupThree.Request) (*GroupThree.Ack, error) {
	log.Printf("%v recieved a request from %v.\n", n.id, req.Id)
	fmt.Println("Recieved a request")

	//checks if state is higher, if equal check id. (This is the hierarchy)
	if n.state == HELD || (n.state == REQUESTED && (n.id > req.Id)) {
		log.Printf("%v is queueing a request from %v\n", n.id, req.Id)
		fmt.Println("Queueing request")
		n.queue = append(n.queue, req.Id)
	} else {
		if n.state == REQUESTED {
			n.requests++
			n.clients[req.Id].Request(ctx, &GroupThree.Request{Id: n.id})
		}
		log.Printf("%v is sending a reply to %v\n", n.id, req.Id)
		fmt.Println("Sending reply")
		n.clients[req.Id].Reply(ctx, &GroupThree.Reply{})
	}
	reply := &GroupThree.Ack{}
	return reply, nil
}

// reply function to check wether all nodes have replied to the request.
func (n *node) Reply(ctx context.Context, req *GroupThree.Reply) (*GroupThree.AckReply, error) {
	n.requests--

	if n.requests == 0 {
		log.Printf("%v recieved a reply. All requests has been replied to.\n", n.id)
		fmt.Println("Recieved reply, all request have been replied to")
		go n.CriticalService()
	}

	log.Printf("%v recieved a reply. Missing %v replies.\n", n.id, n.requests)
	fmt.Println("Recieved a reply, waiting for the remaining replies.")

	rep := &GroupThree.AckReply{}
	return rep, nil
}

// critical service emulated in this method. takes 5 seconds to exec
func (n *node) CriticalService() {
	log.Printf("Critical service accessed by %v\n", n.id)
	fmt.Println("Critical service accessed")
	n.state = HELD

	time.Sleep(5 * time.Second)

	n.state = RELEASED

	log.Printf("%v is done with the critical service, releasing.\n", n.id)
	fmt.Println("Done with the critical service")

	n.ReplyQueue()
}

// Requests all other nodes.
func (n *node) sendRequestToAll() {
	n.state = REQUESTED

	request := &GroupThree.Request{
		Id: n.id,
	}

	n.requests = len(n.clients)

	log.Printf("%v is sending request to all other nodes. Missing %d replies.\n", n.id, n.requests)
	fmt.Println("Sending request to all other nodes")

	for id, client := range n.clients {
		_, err := client.Request(n.ctx, request)

		if err != nil {
			log.Printf("Something went wrong with node: %v, error: %v\n", id, err)
		}
	}
}

// Sends replies to all requests in the queue, then emptied the queue
func (n *node) ReplyQueue() {
	reply := &GroupThree.Reply{}

	for _, id := range n.queue {
		_, err := n.clients[id].Reply(n.ctx, reply)

		if err != nil {
			log.Printf("Something went wrong with node %v, error: %v\n", id, err)
		}
	}
	n.queue = make([]int32, 0)
}

func main() {
	f, err := os.OpenFile("log.server", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v\n", err)
	}
	defer f.Close()
	log.SetOutput(f)

	flag.Parse() //set port with -port in the commandline when running the program

	ctx_, cancel := context.WithCancel(context.Background())
	defer cancel()

	var port = int32(*port)

	//create a node for this proccess
	n := &node{
		id:       port,
		clients:  make(map[int32]GroupThree.Group3Client),
		queue:    make([]int32, 0),
		ctx:      ctx_,
		state:    RELEASED,
		requests: 0,
	}

	//creates listener on port
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v\n", err)
	}

	log.Printf("Node created on port: %d\n", n.id)
	fmt.Printf("Node created on port %v\n", n.id)

	grpcServer := grpc.NewServer()
	GroupThree.RegisterRingServer(grpcServer, n)

	//serve on the listener
	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v\n", err)
		}
	}()

	//for loop to dial all other nodes in the network, if this loop is increased the number of nodes in the network is aswell
	for i := 0; i < 3; i++ {
		nodePort := int32(5000 + i)

		if nodePort == n.id {
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
		c := GroupThree.NewRingClient(conn)
		n.clients[nodePort] = c
	}

	log.Printf("%v is connected to %v other nodes\n", n.id, len(n.clients))
	fmt.Printf("%v is connected to %v other nodes\n", n.id, len(n.clients))

	scanner := bufio.NewScanner(os.Stdin)

	//scanner that requests access from the given node when something is written in the terminal.
	for scanner.Scan() {
		log.Printf("%v is requesting access to critical service...\n", n.id)
		fmt.Println("Requesting acccess to the critical service")
		n.sendRequestToAll()
	}
}
