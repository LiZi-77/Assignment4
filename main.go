package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	GroupThree "github.com/LiZi-77/PCPP-Assignment4/grpc"
	"google.golang.org/grpc"
)

// var port = flag.Int("port", 8000, "port") //port for the node default 8000

const INITPORT int32 = 8000

type STATE int32 //state of the node

const (
	RELEASED STATE = iota
	HELD
	REQUESTED
)

type node struct {
	GroupThree.UnimplementedGroup3Server
	requests int
	id       int32 //port
	clients  map[int32]GroupThree.Group3Client
	ctx      context.Context
	queue    []int32
	state    STATE
}

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	var port = int32(arg1) + INITPORT

	logfile, err := os.OpenFile("log.server", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}
	defer logfile.Close()
	log.SetOutput(logfile)

	flag.Parse() //parse the flags

	ctx_, cancel := context.WithCancel(context.Background())
	defer cancel()

	//create a new node
	_n := &node{
		id:       port,
		clients:  make(map[int32]GroupThree.Group3Client),
		queue:    make([]int32, 0),
		ctx:      ctx_,
		state:    RELEASED,
		requests: 0,
	}

	list, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("Listening on port: %v\n", port)
	fmt.Println("Listening on port: ", port)

	// Create a new gRPC server
	_s := grpc.NewServer()
	GroupThree.RegisterGroup3Server(_s, _n)

	go func() {
		if err := _s.Serve(list); err != nil {
			log.Fatalf("failed to server %v\n", err)
		}
	}()

	//connect to all other nodes
	for i := 0; i < 3; i++ {
		nPort := int32(i) + INITPORT

		if nPort == _n.id {
			continue
		}

		var conn *grpc.ClientConn

		log.Printf("Dialing %v\n", nPort)
		fmt.Println("Dialing ", nPort)

		conn, err := grpc.Dial(fmt.Sprintf(":%v", nPort), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Dial failed %v\n", err)
		}

		defer conn.Close()
		log.Printf("Succes connecting to: %v\n", nPort)
		fmt.Println("Succes connecting to: ", nPort)

		//create a new client
		_c := GroupThree.NewGroup3Client(conn)
		_n.clients[nPort] = _c
	}

	log.Printf("%v is connected to %v other nodes\n", _n.id, len(_n.clients))
	fmt.Printf("%v is connected to %v other nodes\n", _n.id, len(_n.clients))

	// wait for user input
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		if scanner.Text() == "request" {
			if _n.state == RELEASED {
				_n.state = REQUESTED
				_n.requests++
				log.Printf("Node %v is requesting the token\n", _n.id)
				fmt.Printf("Node %v is requesting the token\n", _n.id)

				for _, client := range _n.clients {
					go func(client GroupThree.Group3Client) {
						_, err := client.RequestToken(_n.ctx, &GroupThree.RequestTokenRequest{
							Id: _n.id,
						})
						if err != nil {
							log.Printf("Error requesting token: %v\n", err)
						}
					}(client)
				}
			} else {
				log.Printf("Node %v is already holding the token\n", _n.id)
				fmt.Printf("Node %v is already holding the token\n", _n.id)
			}
		} else if scanner.Text() == "release" {
			if _n.state == HELD {
				_n.state = RELEASED
				log.Printf("Node %v is releasing the token\n", _n.id)
				fmt.Printf("Node %v is releasing the token\n", _n.id)
				_n.ReplyQueue()
			} else {
				log.Printf("Node %v is not holding the token\n", _n.id)
				fmt.Printf("Node %v is not holding the token\n", _n.id)
			}
		} else {
			log.Printf("Invalid command: %v\n", scanner.Text())
			fmt.Printf("Invalid command: %v\n", scanner.Text())
		}
	}
	// log.Printf("%v is requesting access to critical service...\n", _n.id)
	// fmt.Println("Requesting acccess to the critical service")
	// _n.sendRequestToAll()
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
