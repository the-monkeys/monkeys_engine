
import grpc
from concurrent import futures
from gw_recom_pb2 import RecommendationRes
import gw_recom_pb2_grpc
import yaml

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

# Start the gRPC server
def serve():
    # Read the config.yaml file
    try:
        with open("config/config.yaml", "r") as file:
            config = yaml.safe_load(file)
            
            # Extract the recommendation engine address from config
            recomm_address = config["microservices"]["the_monkeys_recomm_engine"]
            # Split host and port for Docker compatibility
            host, port = recomm_address.split(":")
            # Bind to 0.0.0.0 inside Docker container
            server_address = f"0.0.0.0:{port}"
            print(f"✅ Starting recommendation engine server on {server_address} (config: {recomm_address})")

            # Create and start the gRPC server
            server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
            gw_recom_pb2_grpc.add_RecommendationServiceServicer_to_server(
                RecommendationServiceServicer(), server
            )
            server.add_insecure_port(server_address)  # Use 0.0.0.0 for Docker
            print(f"Server is running on {recomm_address}...")
            server.start()
            server.wait_for_termination()
            
    except FileNotFoundError:
        print("Error: 'config/config.yaml' file not found. Please ensure the file exists.")
        return
    except KeyError as e:
        print(f"Error: Missing key in configuration: {e}")
        return

if __name__ == "__main__":
    try:
        serve()
    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()