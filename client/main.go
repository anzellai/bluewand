package main

import (
	"context"
	"flag"
	"io"
	"log"
	"time"

	pb "github.com/anzellai/bluewand/bluewand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	tls                = flag.Bool("tls", false, "Connection to use TLS if set true, otherwise plain TCP is used")
	cert               = flag.String("cert", "", "TLS cert file path if TLS is used")
	serverAddr         = flag.String("server_addr", "127.0.0.1:55555", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "me.kano.grpc.bluewand", "The server name use to verify the hostname returned by TLS handshake")
	wand               = &pb.Identifier{} // variable holding the Wand UID returned by server
)

func onConnect(client pb.BlueWandClient, empty *pb.EmptyMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	identifer, err := client.OnConnect(ctx, empty)
	if err != nil {
		log.Fatalf("%v.OnConnect(_) = _, %v", client, err)
	}
	wand = identifer
	log.Printf("Identifier: %s", identifer.Uid)
}

func onButton(client pb.BlueWandClient, identifier *pb.Identifier) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.OnButton(ctx, identifier)
	if err != nil {
		log.Fatalf("%v.OnButton(_) = _, %v", client, err)
	}
	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("failed to receive OnButton event: %v", err)
			}
			log.Printf("OnButton: %t", in.Pressed)
		}
	}()
	stream.CloseSend()
	<-waitc
}

func onMotion(client pb.BlueWandClient, identifier *pb.Identifier) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.OnMotion(ctx, identifier)
	if err != nil {
		log.Fatalf("%v.OnMotion(_) = _, %v", client, err)
	}
	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("failed to receive OnButton event: %v", err)
			}
			log.Printf("OnMotion: [%d, %d, %d, %d]", in.W, in.X, in.Y, in.Z)
		}
	}()
	stream.CloseSend()
	<-waitc
}

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	if *tls {
		if *cert == "" {
			log.Fatalln("missing ca cert file")
		}
		creds, err := credentials.NewClientTLSFromFile(*cert, *serverHostOverride)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewBlueWandClient(conn)
	onConnect(client, &pb.EmptyMessage{})
	onButton(client, wand)
}
