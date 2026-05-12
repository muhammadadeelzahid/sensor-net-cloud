package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/soheilhy/cmux"
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

	// Start listening
	listener, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	// Set up cmux
	m := cmux.New(listener)
	grpcL := m.Match(cmux.HTTP2())
	httpL := m.Match(cmux.HTTP1Fast())

	// Set up HTTP health endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	httpSrv := &http.Server{
		Handler: mux,
	}

	// Start servers
	go func() {
		log.Printf("Starting gRPC server on port %s...", port)
		if err := grpcSrv.Serve(grpcL); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	go func() {
		log.Printf("Starting HTTP server on port %s...", port)
		if err := httpSrv.Serve(httpL); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	// Start cmux
	if err := m.Serve(); err != nil {
		log.Fatalf("cmux serve error: %v", err)
	}
}
