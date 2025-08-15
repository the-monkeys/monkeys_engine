
import grpc
from concurrent import futures
from gw_recom_pb2 import RecommendationRes
import gw_recom_pb2_grpc
import os
from dotenv import load_dotenv
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

# Health check handler for HTTP endpoint
class HealthHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/health' or self.path == '/healthz':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            health_status = {
                "status": "healthy",
                "service": "the_monkeys_ai",
                "timestamp": time.time(),
                "uptime_seconds": time.time() - server_start_time
            }
            self.wfile.write(json.dumps(health_status).encode())
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b'Not Found')
    
    def log_message(self, format, *args):
        # Suppress HTTP server logs to keep output clean
        pass

# Global variable to track server start time
server_start_time = time.time()

# Implement the RecommendationServiceServicer
class RecommendationServiceServicer(gw_recom_pb2_grpc.RecommendationServiceServicer):
    def GetRecommendations(self, request, context):
        # Log the incoming request
        print(f"Received request for user: {request.username}")

        # Example logic to generate recommendations
        topics = ["Technology", "Science", "Music"]
        users_to_follow = ["user123", "user456", "user789"]
        posts_to_read = []  # This can be populated with `google.protobuf.Any` objects

        # Create and return the response
        return RecommendationRes(
            topics=topics,
            users_to_follow=users_to_follow,
            posts_to_read=posts_to_read
        )

def start_health_server(health_port=8080):
    """Start HTTP health check server in a separate thread"""
    try:
        health_server = HTTPServer(('0.0.0.0', health_port), HealthHandler)
        print(f"🏥 Health check server started on port {health_port}")
        health_server.serve_forever()
    except Exception as e:
        print(f"⚠️  Failed to start health server: {e}")

# Start the gRPC server
def serve():
    global server_start_time
    server_start_time = time.time()
    
    # Load environment variables from .env file
    try:
        # Try to load .env file from project root
        env_path = os.path.join(os.path.dirname(__file__), '..', '..', '.env')
        if os.path.exists(env_path):
            load_dotenv(env_path)
            print(f"✅ Loaded environment variables from {env_path}")
        else:
            print("⚠️  .env file not found, using system environment variables")
        
        # Get ports directly from environment variables
        grpc_port = int(os.getenv("MICROSERVICES_AI_ENGINE_INTERNAL_PORT", "50057"))
        health_port = int(os.getenv("MICROSERVICES_AI_ENGINE_HEALTH_INTERNAL_PORT", "51057"))
        
        # Server addresses - bind to 0.0.0.0 inside Docker container
        server_address = f"0.0.0.0:{grpc_port}"
        
        print(f"✅ Starting recommendation engine server on {server_address}")
        print(f"🏥 Health check will be available on port {health_port}")

        # Start health check server in background thread
        health_thread = threading.Thread(target=start_health_server, args=(health_port,), daemon=True)
        health_thread.start()

        # Create and start the gRPC server
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        gw_recom_pb2_grpc.add_RecommendationServiceServicer_to_server(
            RecommendationServiceServicer(), server
        )
        server.add_insecure_port(server_address)
        print(f"🚀 gRPC server is running on {server_address}")
        print(f"🏥 Health check available at http://0.0.0.0:{health_port}/health")
        server.start()
        server.wait_for_termination()
        
    except Exception as e:
        print(f"Error loading configuration: {e}")
        print("Using default configuration...")
        # Fallback to default values
        server_address = "0.0.0.0:50057"
        health_port = 51057
        
        print(f"✅ Starting recommendation engine server on {server_address} (default)")

        # Start health check server in background thread
        health_thread = threading.Thread(target=start_health_server, args=(health_port,), daemon=True)
        health_thread.start()

        # Create and start the gRPC server
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        gw_recom_pb2_grpc.add_RecommendationServiceServicer_to_server(
            RecommendationServiceServicer(), server
        )
        server.add_insecure_port(server_address)
        print(f"🚀 gRPC server is running on {server_address}...")
        print(f"🏥 Health check available at http://0.0.0.0:{health_port}/health")
        server.start()
        server.wait_for_termination()

if __name__ == "__main__":
    try:
        serve()
    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()