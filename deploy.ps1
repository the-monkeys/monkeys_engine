# The Monkeys Engine Deployment Script for Windows PowerShell
# Automates deployment with health monitoring and optimization

param(
    [Parameter(HelpMessage = "Deployment environment (development|production)")]
    [ValidateSet("development", "production")]
    [string]$Environment = "development",
    
    [Parameter(HelpMessage = "Force rebuild all containers")]
    [switch]$Build,
    
    [Parameter(HelpMessage = "Force deployment (remove existing containers)")]
    [switch]$Force,
    
    [Parameter(HelpMessage = "Enable service scaling")]
    [switch]$Scale,
    
    [Parameter(HelpMessage = "Enable verbose logging")]
    [switch]$VerboseLogging,
    
    [Parameter(HelpMessage = "Skip health checks")]
    [switch]$NoHealth,
    
    [Parameter(HelpMessage = "Show help message")]
    [switch]$Help
)

# Script configuration
$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectName = "the_monkeys_engine"
$LogFile = Join-Path $ScriptDir "logs\deploy-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"

# Create logs directory
$LogsDir = Join-Path $ScriptDir "logs"
if (-not (Test-Path $LogsDir)) {
    New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null
}

# Color configuration for console output
$Colors = @{
    Info  = "Green"
    Warn  = "Yellow"
    Error = "Red"
    Debug = "Cyan"
}

# Logging function
function Write-Log {
    param(
        [Parameter(Mandatory = $true)]
        [ValidateSet("Info", "Warn", "Error", "Debug")]
        [string]$Level,
        
        [Parameter(Mandatory = $true)]
        [string]$Message
    )
    
    $Timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $LogEntry = "[$Timestamp] [$Level] $Message"
    
    # Write to console with color
    $Color = $Colors[$Level]
    if ($Level -eq "Debug" -and -not $VerboseLogging) {
        # Skip debug messages unless verbose is enabled
    }
    else {
        Write-Host "[$Level] $Message" -ForegroundColor $Color
    }
    
    # Write to log file
    Add-Content -Path $LogFile -Value $LogEntry
}

# Help function
function Show-Help {
    Write-Host @"
The Monkeys Engine Deployment Script for Windows PowerShell

USAGE:
    .\deploy.ps1 [OPTIONS]

OPTIONS:
    -Environment    Set deployment environment (development|production) [default: development]
    -Build          Force rebuild all containers
    -Force          Force deployment (remove existing containers)
    -Scale          Enable service scaling
    -VerboseLogging Enable verbose logging
    -NoHealth       Skip health checks
    -Help           Show this help message

EXAMPLES:
    .\deploy.ps1                                    # Deploy development environment
    .\deploy.ps1 -Environment production -Build     # Build and deploy production
    .\deploy.ps1 -Force -Scale -VerboseLogging       # Force deploy with scaling and verbose logs
    
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

"@
}

# Check prerequisites
function Test-Prerequisites {
    Write-Log "Info" "Checking prerequisites..."
    
    # Check Docker
    try {
        $null = docker --version
        Write-Log "Debug" "Docker found: $(docker --version)"
    }
    catch {
        Write-Log "Error" "Docker is not installed or not in PATH"
        exit 1
    }
    
    # Check Docker Compose
    try {
        $null = docker-compose --version
        Write-Log "Debug" "Docker Compose found: $(docker-compose --version)"
    }
    catch {
        Write-Log "Error" "Docker Compose is not installed or not in PATH"
        exit 1
    }
    
    # Check if Docker daemon is running
    Write-Log "Debug" "Testing Docker daemon connection..."
    try {
        $null = docker ps 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Log "Debug" "Docker daemon is running (Docker Desktop)"
        }
        else {
            Write-Log "Error" "Docker daemon is not running. Please start Docker Desktop."
            exit 1
        }
    }
    catch {
        Write-Log "Error" "Docker daemon check failed: $_"
        Write-Log "Error" "Please ensure Docker Desktop is running."
        exit 1
    }
    
    # Check environment file
    if (-not (Test-Path ".env")) {
        Write-Log "Warn" ".env file not found. Creating from template..."
        if (Test-Path ".env.example") {
            Copy-Item ".env.example" ".env"
            Write-Log "Info" "Created .env from .env.example. Please review and update configuration."
        }
        else {
            Write-Log "Error" ".env.example not found. Cannot create environment configuration."
            exit 1
        }
    }
    
    # Set compose file based on environment
    if ($Environment -eq "production") {
        $script:ComposeFile = "docker-compose.prod.yml"
        Write-Log "Info" "Using production configuration"
    }
    else {
        $script:ComposeFile = "docker-compose.yml"
        Write-Log "Info" "Using development configuration"
    }
    
    # Check compose file
    if (-not (Test-Path $script:ComposeFile)) {
        Write-Log "Error" "Compose file not found: $script:ComposeFile"
        exit 1
    }
    
    Write-Log "Info" "Prerequisites check completed successfully"
}

# Clean up existing containers
function Invoke-Cleanup {
    if ($Force) {
        Write-Log "Info" "Force flag detected. Cleaning up existing containers..."
        try {
            docker-compose -f $script:ComposeFile down -v 2>$null
            docker system prune -f 2>$null
            Write-Log "Info" "Cleanup completed"
        }
        catch {
            Write-Log "Warn" "Cleanup encountered errors (this is usually safe to ignore)"
        }
    }
}

# Build containers
function Invoke-Build {
    if ($Build) {
        Write-Log "Info" "Building containers (this may take several minutes)..."
        
        try {
            if ($VerboseLogging) {
                docker-compose -f $script:ComposeFile build --no-cache
            }
            else {
                docker-compose -f $script:ComposeFile build --no-cache 2>$null
            }
            Write-Log "Info" "Container build completed"
        }
        catch {
            Write-Log "Error" "Container build failed: $_"
            exit 1
        }
    }
}

# Deploy infrastructure services
function Deploy-Infrastructure {
    Write-Log "Info" "Deploying infrastructure services..."
    
    $InfrastructureServices = @(
        "the_monkeys_db",
        "elasticsearch-node1",
        "rabbitmq",
        "the_monkeys_cache",
        "minio"
    )
    
    foreach ($Service in $InfrastructureServices) {
        Write-Log "Debug" "Starting $Service..."
        try {
            docker-compose -f $script:ComposeFile up -d $Service
        }
        catch {
            Write-Log "Error" "Failed to start ${Service}: $_"
            exit 1
        }
    }
    
    Write-Log "Info" "Infrastructure services started. Waiting for readiness..."
    Start-Sleep -Seconds 30
}

# Deploy microservices
function Deploy-Microservices {
    Write-Log "Info" "Deploying microservices..."
    
    $Microservices = @(
        "the_monkeys_authz",
        "the_monkeys_blog",
        "the_monkeys_user",
        "the_monkeys_storage",
        "the_monkeys_notification",
        "the_monkeys_gateway",
        "the_monkeys_ai_engine"
    )
    
    foreach ($Service in $Microservices) {
        Write-Log "Debug" "Starting $Service..."
        try {
            docker-compose -f $script:ComposeFile up -d $Service
            
            if ($Scale -and $Service -ne "the_monkeys_ai_engine") {
                switch ($Service) {
                    { $_ -in @("the_monkeys_authz", "the_monkeys_gateway") } {
                        Write-Log "Debug" "Scaling $Service to 2 replicas..."
                        docker-compose -f $script:ComposeFile up -d --scale "$Service=2" $Service
                    }
                }
            }
        }
        catch {
            Write-Log "Error" "Failed to start ${Service}: $_"
            exit 1
        }
    }
    
    Write-Log "Info" "Microservices deployment completed"
}

# Health check functions
function Test-GrpcHealth {
    param(
        [string]$ServiceName,
        [int]$Port,
        [string]$ContainerName
    )
    
    Write-Log "Debug" "Checking gRPC health for $ServiceName on port $Port..."
    
    try {
        $result = docker exec $ContainerName grpc_health_probe -addr=":$Port" 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-Log "Info" "✅ $ServiceName health check: SERVING"
            return $true
        }
        else {
            Write-Log "Warn" "❌ $ServiceName health check: FAILED"
            return $false
        }
    }
    catch {
        Write-Log "Warn" "❌ $ServiceName health check: FAILED"
        return $false
    }
}

function Test-HttpHealth {
    param(
        [string]$ServiceName,
        [string]$Url
    )
    
    Write-Log "Debug" "Checking HTTP health for $ServiceName at $Url..."
    
    try {
        $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
        if ($response.StatusCode -eq 200) {
            Write-Log "Info" "✅ $ServiceName health check: OK"
            return $true
        }
        else {
            Write-Log "Warn" "❌ $ServiceName health check: FAILED"
            return $false
        }
    }
    catch {
        Write-Log "Warn" "❌ $ServiceName health check: FAILED"
        return $false
    }
}

# Comprehensive health checks
function Invoke-HealthChecks {
    if ($NoHealth) {
        Write-Log "Info" "Health checks skipped (-NoHealth flag)"
        return $true
    }
    
    Write-Log "Info" "Running health checks..."
    
    # Wait for services to initialize
    Write-Log "Info" "Waiting for services to initialize..."
    Start-Sleep -Seconds 15
    
    $FailedChecks = 0
    
    # gRPC health checks
    if (-not (Test-GrpcHealth "Authorization" 50051 "the_monkeys_authz")) { $FailedChecks++ }
    if (-not (Test-GrpcHealth "Blog" 50052 "the_monkeys_blog")) { $FailedChecks++ }
    if (-not (Test-GrpcHealth "User" 50053 "the_monkeys_user")) { $FailedChecks++ }
    if (-not (Test-GrpcHealth "Storage" 50054 "the_monkeys_storage")) { $FailedChecks++ }
    if (-not (Test-GrpcHealth "Notification" 50055 "the_monkeys_notification")) { $FailedChecks++ }
    
    # HTTP health checks
    if (-not (Test-HttpHealth "Gateway" "http://localhost:8081/healthz")) { $FailedChecks++ }
    if (-not (Test-HttpHealth "AI Engine" "http://localhost:51057/health")) { $FailedChecks++ }
    
    if ($FailedChecks -eq 0) {
        Write-Log "Info" "🎉 All health checks passed! Deployment successful."
        return $true
    }
    else {
        Write-Log "Warn" "⚠️  $FailedChecks health check(s) failed. Check service logs."
        return $false
    }
}

# Show deployment status
function Show-Status {
    Write-Log "Info" "Deployment Status:"
    Write-Host ""
    
    # Container status
    Write-Host "Container Status:" -ForegroundColor Cyan
    try {
        $containers = docker ps --format "table {{.Names}}`t{{.Status}}`t{{.Ports}}" | Select-String -Pattern "(the_monkeys|elasticsearch|rabbitmq)"
        $containers | ForEach-Object { Write-Host $_.Line }
    }
    catch {
        Write-Log "Warn" "Could not retrieve container status"
    }
    Write-Host ""
    
    # Service endpoints
    Write-Host "Service Endpoints:" -ForegroundColor Cyan
    Write-Host "  Gateway:        http://localhost:8081"
    Write-Host "  Authorization:  grpc://localhost:50051"
    Write-Host "  Blog Service:   grpc://localhost:50052"
    Write-Host "  User Service:   grpc://localhost:50053"
    Write-Host "  Storage:        grpc://localhost:50054"
    Write-Host "  Notification:   grpc://localhost:50055"
    Write-Host "  AI Engine:      http://localhost:51057"
    Write-Host ""
    
    # Quick commands
    Write-Host "Quick Commands:" -ForegroundColor Cyan
    Write-Host "  View logs:      docker-compose -f $script:ComposeFile logs -f"
    Write-Host "  Health check:   Invoke-WebRequest http://localhost:8081/healthz"
    Write-Host "  Stop services:  docker-compose -f $script:ComposeFile down"
    Write-Host "  Resource usage: docker stats"
    Write-Host ""
}

# Main deployment function
function Invoke-Main {
    if ($Help) {
        Show-Help
        return
    }
    
    Write-Log "Info" "Starting The Monkeys Engine deployment..."
    Write-Log "Info" "Environment: $Environment"
    Write-Log "Info" "Build: $(if ($Build) { 'enabled' } else { 'disabled' })"
    Write-Log "Info" "Force: $(if ($Force) { 'enabled' } else { 'disabled' })"
    Write-Log "Info" "Scaling: $(if ($Scale) { 'enabled' } else { 'disabled' })"
    Write-Log "Info" "Log file: $LogFile"
    Write-Host ""
    
    try {
        # Execute deployment steps
        Test-Prerequisites
        Invoke-Cleanup
        Invoke-Build
        Deploy-Infrastructure
        Deploy-Microservices
        
        # Run health checks and show status
        if (Invoke-HealthChecks) {
            Show-Status
            Write-Log "Info" "🚀 Deployment completed successfully!"
            
            if ($Environment -eq "production") {
                Write-Log "Info" "Production deployment ready. Monitor health and performance."
            }
            else {
                Write-Log "Info" "Development environment ready for coding!"
            }
        }
        else {
            Write-Log "Error" "Deployment completed with health check failures. Review logs."
            exit 1
        }
    }
    catch {
        Write-Log "Error" "Deployment failed: $_"
        exit 1
    }
}

# Handle Ctrl+C gracefully
$null = Register-EngineEvent PowerShell.Exiting -Action {
    Write-Log "Error" "Deployment interrupted"
}

# Run main function
Invoke-Main