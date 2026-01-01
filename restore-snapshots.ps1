# PowerShell script to restore Elasticsearch snapshots from backup

Write-Host "Starting Elasticsearch snapshot restoration process..." -ForegroundColor Green

# Wait for Elasticsearch to be ready
Write-Host "Waiting for Elasticsearch to be ready..." -ForegroundColor Yellow
do {
    try {
        Invoke-RestMethod -Uri "http://localhost:9200/_cluster/health" -Method Get -ErrorAction Stop | Out-Null
        break
    }
    catch {
        Write-Host "   Elasticsearch not ready yet, waiting 5 seconds..." -ForegroundColor Gray
        Start-Sleep -Seconds 5
    }
} while ($true)

Write-Host "Elasticsearch is ready!" -ForegroundColor Green

# Register the snapshot repository
Write-Host "Registering snapshot repository..." -ForegroundColor Cyan
$repoBody = @{
    type     = "fs"
    settings = @{
        location = "/usr/share/elasticsearch/snapshots"
    }
} | ConvertTo-Json

try {
    Invoke-RestMethod -Uri "http://localhost:9200/_snapshot/backup_repo" -Method Put -Body $repoBody -ContentType "application/json"
    Write-Host "Repository registered successfully!" -ForegroundColor Green
}
catch {
    Write-Host "Failed to register repository: $($_.Exception.Message)" -ForegroundColor Red
}

# Check available snapshots
Write-Host "Checking available snapshots..." -ForegroundColor Cyan
try {
    $snapshots = Invoke-RestMethod -Uri "http://localhost:9200/_snapshot/backup_repo/_all?pretty" -Method Get
    
    # Get the latest snapshot
    if ($snapshots.snapshots -and $snapshots.snapshots.Count -gt 0) {
        $latestSnapshot = $snapshots.snapshots[-1].snapshot
        Write-Host "Latest snapshot found: $latestSnapshot" -ForegroundColor Green
        
        # Restore the latest snapshot
        Write-Host "Restoring snapshot: $latestSnapshot" -ForegroundColor Cyan
        
        # Close all indices first to avoid conflicts
        Write-Host "Closing existing indices..." -ForegroundColor Yellow
        try {
            $indices = Invoke-RestMethod -Uri "http://localhost:9200/_cat/indices?format=json&expand_wildcards=all" -Method Get
            foreach ($index in $indices) {
                $name = $index.index
                Write-Host "Closing index: $name" -ForegroundColor Gray
                try {
                    Invoke-RestMethod -Uri "http://localhost:9200/$name/_close" -Method Post
                }
                catch {
                    Write-Host "Failed to close ${name}: $($_.Exception.Message)" -ForegroundColor Red
                }
            }
        }
        catch {
            Write-Host "Warning: Failed to list/close indices: $($_.Exception.Message)" -ForegroundColor Gray
        }

        $restoreBody = @{
            ignore_unavailable   = $true
            include_global_state = $false
        } | ConvertTo-Json
        
        try {
            Invoke-RestMethod -Uri "http://localhost:9200/_snapshot/backup_repo/$latestSnapshot/_restore?pretty" -Method Post -Body $restoreBody -ContentType "application/json"
            Write-Host "Restoration initiated successfully!" -ForegroundColor Green
            
            Write-Host "Waiting for restoration to complete..." -ForegroundColor Yellow
            Start-Sleep -Seconds 15
            
            # Check available indices
            Write-Host "Checking available indices..." -ForegroundColor Cyan
            $indices = Invoke-WebRequest -Uri "http://localhost:9200/_cat/indices?v" -Method Get -UseBasicParsing
            Write-Host $indices.Content -ForegroundColor White
            
            Write-Host "Snapshot restoration process completed!" -ForegroundColor Green
            Write-Host "Your Elasticsearch data should now be available!" -ForegroundColor Green
        }
        catch {
            Write-Host "Failed to restore snapshot: $($_.Exception.Message)" -ForegroundColor Red
        }
    }
    else {
        Write-Host "No snapshots found in the repository!" -ForegroundColor Red
    }
}
catch {
    Write-Host "Failed to check snapshots: $($_.Exception.Message)" -ForegroundColor Red
}
