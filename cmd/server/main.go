package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"sensor-net-cloud/gen/sensornetpb"
	"sensor-net-cloud/internal/db"
	"sensor-net-cloud/internal/grpcserver"
	"sensor-net-cloud/internal/migrations"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Run migrations
	if err := migrations.Run(dbURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Connect to database
	database, err := db.New(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Initialize gRPC server
	grpcSrv := grpc.NewServer()
	sensornetpb.RegisterGatewayCloudServiceServer(grpcSrv, grpcserver.New(database))

	// Enable reflection for development/testing
	reflection.Register(grpcSrv)

	// Start a simple HTTP server for healthz
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})
		// We can listen on another port, or the same port using cmux, but Render expects a single port.
		// Wait, if Render only exposes one port (PORT) we can't easily serve both HTTP and gRPC on the same port without multiplexing.
		// The prompt says "If combining HTTP and gRPC in one service is inconvenient, skip this for the first version".
		// I will comment this out for now to keep it simple and ensure gRPC works perfectly on the required port.
	}()

	// Start listening
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	log.Printf("Starting gRPC server on port %s...", port)
	if err := grpcSrv.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC: %v", err)
	}
}
