#!/bin/bash
# The Monkeys Engine Deployment Script for Linux/macOS
# Automates deployment with health monitoring and optimization

set -euo pipefail

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="the_monkeys_engine"
LOG_FILE="${SCRIPT_DIR}/logs/deploy-$(date +%Y%m%d-%H%M%S).log"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default configuration
ENVIRONMENT="development"
BUILD_FLAG=false
FORCE_FLAG=false
SCALE_SERVICES=false
VERBOSE=false
HEALTH_CHECK=true

# Create logs directory
mkdir -p "${SCRIPT_DIR}/logs"

# Logging function
log() {
    local level=$1
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        "INFO")  echo -e "${GREEN}[INFO]${NC} $message" | tee -a "$LOG_FILE" ;;
        "WARN")  echo -e "${YELLOW}[WARN]${NC} $message" | tee -a "$LOG_FILE" ;;
        "ERROR") echo -e "${RED}[ERROR]${NC} $message" | tee -a "$LOG_FILE" ;;
        "DEBUG") [[ $VERBOSE == true ]] && echo -e "${BLUE}[DEBUG]${NC} $message" | tee -a "$LOG_FILE" ;;
    esac
}

# Help function
show_help() {
    cat << EOF
The Monkeys Engine Deployment Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -e, --env ENVIRONMENT     Set deployment environment (development|production) [default: development]
    -b, --build              Force rebuild all containers
    -f, --force              Force deployment (remove existing containers)
    -s, --scale              Enable service scaling
    -v, --verbose            Enable verbose logging
    --no-health              Skip health checks
    -h, --help               Show this help message

EXAMPLES:
    $0                                   # Deploy development environment
    $0 --env production --build          # Build and deploy production
    $0 --force --scale --verbose         # Force deploy with scaling and verbose logs
    
ENVIRONMENT FILES:
    The script requires a .env file with proper configuration.
    Copy .env.example to .env and configure your settings.

SERVICES:
    - Gateway (HTTP/gRPC)     : http://localhost:8081
    - Authorization (gRPC)    : localhost:50051
    - Blog Service (gRPC)     : localhost:50052
    - User Service (gRPC)     : localhost:50053
    - Storage Service (gRPC)  : localhost:50054
    - Notification (gRPC)     : localhost:50055
    - AI Engine (HTTP)       : http://localhost:51057

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--env)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -b|--build)
            BUILD_FLAG=true
            shift
            ;;
        -f|--force)
            FORCE_FLAG=true
            shift
            ;;
        -s|--scale)
            SCALE_SERVICES=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        --no-health)
            HEALTH_CHECK=false
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            log "ERROR" "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Validate environment
if [[ ! "$ENVIRONMENT" =~ ^(development|production)$ ]]; then
    log "ERROR" "Invalid environment: $ENVIRONMENT. Must be 'development' or 'production'"
    exit 1
fi

# Set compose file based on environment
if [[ "$ENVIRONMENT" == "production" ]]; then
    COMPOSE_FILE="docker-compose.prod.yml"
    log "INFO" "Using production configuration"
else
    COMPOSE_FILE="docker-compose.yml"
    log "INFO" "Using development configuration"
fi

# Check prerequisites
check_prerequisites() {
    log "INFO" "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log "ERROR" "Docker is not installed or not in PATH"
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        log "ERROR" "Docker Compose is not installed or not in PATH"
        exit 1
    fi
    
    # Check if Docker daemon is running
    if ! docker info &> /dev/null; then
        log "ERROR" "Docker daemon is not running"
        exit 1
    fi
    
    # Check environment file
    if [[ ! -f ".env" ]]; then
        log "WARN" ".env file not found. Creating from template..."
        if [[ -f ".env.example" ]]; then
            cp .env.example .env
            log "INFO" "Created .env from .env.example. Please review and update configuration."
        else
            log "ERROR" ".env.example not found. Cannot create environment configuration."
            exit 1
        fi
    fi
    
    # Check compose file
    if [[ ! -f "$COMPOSE_FILE" ]]; then
        log "ERROR" "Compose file not found: $COMPOSE_FILE"
        exit 1
    fi
    
    log "INFO" "Prerequisites check completed successfully"
}

# Clean up existing containers if force flag is set
cleanup() {
    if [[ "$FORCE_FLAG" == true ]]; then
        log "INFO" "Force flag detected. Cleaning up existing containers..."
        docker-compose -f "$COMPOSE_FILE" down -v 2>/dev/null || true
        docker system prune -f &>/dev/null || true
        log "INFO" "Cleanup completed"
    fi
}

# Build containers
build_containers() {
    if [[ "$BUILD_FLAG" == true ]]; then
        log "INFO" "Building containers (this may take several minutes)..."
        
        if [[ "$VERBOSE" == true ]]; then
            docker-compose -f "$COMPOSE_FILE" build --no-cache
        else
            docker-compose -f "$COMPOSE_FILE" build --no-cache &>/dev/null
        fi
        
        log "INFO" "Container build completed"
    fi
}

# Deploy infrastructure services
deploy_infrastructure() {
    log "INFO" "Deploying infrastructure services..."
    
    local infrastructure_services=(
        "the_monkeys_db"
        "elasticsearch-node1"
        "rabbitmq"
        "the_monkeys_cache"
        "minio"
    )
    
    for service in "${infrastructure_services[@]}"; do
        log "DEBUG" "Starting $service..."
        docker-compose -f "$COMPOSE_FILE" up -d "$service"
    done
    
    log "INFO" "Infrastructure services started. Waiting for readiness..."
    sleep 30
}

# Deploy microservices
deploy_microservices() {
    log "INFO" "Deploying microservices..."
    
    local microservices=(
        "the_monkeys_authz"
        "the_monkeys_blog"
        "the_monkeys_user"
        "the_monkeys_storage"
        "the_monkeys_notification"
        "the_monkeys_gateway"
        "the_monkeys_ai_engine"
    )
    
    for service in "${microservices[@]}"; do
        log "DEBUG" "Starting $service..."
        docker-compose -f "$COMPOSE_FILE" up -d "$service"
        
        if [[ "$SCALE_SERVICES" == true && "$service" != "the_monkeys_ai_engine" ]]; then
            case $service in
                "the_monkeys_authz"|"the_monkeys_gateway")
                    log "DEBUG" "Scaling $service to 2 replicas..."
                    docker-compose -f "$COMPOSE_FILE" up -d --scale "$service=2" "$service"
                    ;;
            esac
        fi
    done
    
    log "INFO" "Microservices deployment completed"
}

# Health check functions
check_grpc_health() {
    local service=$1
    local port=$2
    local container_name=$3
    
    log "DEBUG" "Checking gRPC health for $service on port $port..."
    
    if docker exec "$container_name" grpc_health_probe -addr=":$port" &>/dev/null; then
        log "INFO" "✅ $service health check: SERVING"
        return 0
    else
        log "WARN" "❌ $service health check: FAILED"
        return 1
    fi
}

check_http_health() {
    local service=$1
    local url=$2
    
    log "DEBUG" "Checking HTTP health for $service at $url..."
    
    if curl -f -s "$url" &>/dev/null; then
        log "INFO" "✅ $service health check: OK"
        return 0
    else
        log "WARN" "❌ $service health check: FAILED"
        return 1
    fi
}

# Comprehensive health checks
run_health_checks() {
    if [[ "$HEALTH_CHECK" == false ]]; then
        log "INFO" "Health checks skipped (--no-health flag)"
        return 0
    fi
    
    log "INFO" "Running health checks..."
    
    # Wait for services to initialize
    log "INFO" "Waiting for services to initialize..."
    sleep 15
    
    local failed_checks=0
    
    # gRPC health checks
    check_grpc_health "Authorization" "50051" "the_monkeys_authz" || ((failed_checks++))
    check_grpc_health "Blog" "50052" "the_monkeys_blog" || ((failed_checks++))
    check_grpc_health "User" "50053" "the_monkeys_user" || ((failed_checks++))
    check_grpc_health "Storage" "50054" "the_monkeys_storage" || ((failed_checks++))
    check_grpc_health "Notification" "50055" "the_monkeys_notification" || ((failed_checks++))
    
    # HTTP health checks
    check_http_health "Gateway" "http://localhost:8081/healthz" || ((failed_checks++))
    check_http_health "AI Engine" "http://localhost:51057/health" || ((failed_checks++))
    
    if [[ $failed_checks -eq 0 ]]; then
        log "INFO" "🎉 All health checks passed! Deployment successful."
    else
        log "WARN" "⚠️  $failed_checks health check(s) failed. Check service logs."
    fi
    
    return $failed_checks
}

# Show deployment status
show_status() {
    log "INFO" "Deployment Status:"
    echo ""
    
    # Container status
    echo -e "${BLUE}Container Status:${NC}"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "(the_monkeys|elasticsearch|rabbitmq)"
    echo ""
    
    # Service endpoints
    echo -e "${BLUE}Service Endpoints:${NC}"
    cat << EOF
  🌐 Gateway:        http://localhost:8081
  🔐 Authorization:  grpc://localhost:50051
  📝 Blog Service:   grpc://localhost:50052
  👤 User Service:   grpc://localhost:50053
  💾 Storage:        grpc://localhost:50054
  🔔 Notification:   grpc://localhost:50055
  🤖 AI Engine:      http://localhost:51057
EOF
    echo ""
    
    # Quick commands
    echo -e "${BLUE}Quick Commands:${NC}"
    cat << EOF
  📊 View logs:      docker-compose -f $COMPOSE_FILE logs -f
  🔍 Health check:   curl http://localhost:8081/healthz
  🛑 Stop services:  docker-compose -f $COMPOSE_FILE down
  📈 Resource usage: docker stats
EOF
    echo ""
}

# Main deployment function
main() {
    log "INFO" "Starting The Monkeys Engine deployment..."
    log "INFO" "Environment: $ENVIRONMENT"
    log "INFO" "Build: $([ "$BUILD_FLAG" == true ] && echo "enabled" || echo "disabled")"
    log "INFO" "Force: $([ "$FORCE_FLAG" == true ] && echo "enabled" || echo "disabled")"
    log "INFO" "Scaling: $([ "$SCALE_SERVICES" == true ] && echo "enabled" || echo "disabled")"
    log "INFO" "Log file: $LOG_FILE"
    echo ""
    
    # Execute deployment steps
    check_prerequisites
    cleanup
    build_containers
    deploy_infrastructure
    deploy_microservices
    
    # Run health checks and show status
    if run_health_checks; then
        show_status
        log "INFO" "🚀 Deployment completed successfully!"
        
        if [[ "$ENVIRONMENT" == "production" ]]; then
            log "INFO" "Production deployment ready. Monitor health and performance."
        else
            log "INFO" "Development environment ready for coding!"
        fi
    else
        log "ERROR" "Deployment completed with health check failures. Review logs."
        exit 1
    fi
}

# Trap signals for cleanup
trap 'log "ERROR" "Deployment interrupted"; exit 1' INT TERM

# Run main function
main "$@"