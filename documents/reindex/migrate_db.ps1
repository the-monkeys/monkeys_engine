# migrate_db.ps1
# Migrates the_monkeys_blogs to v2 mapping to support nested lists

Write-Host "Starting Blog Migration to V2..." -ForegroundColor Cyan

# 1. Create V2 Index
Write-Host "Creating the_monkeys_blogs_v2 index..."
try {
    # Check if exists first causing error? No, PUT creates or updates. But better delete if exists to be clean or ignore error.
    Invoke-RestMethod -Uri "http://localhost:9200/the_monkeys_blogs_v2" -Method Delete -ErrorAction SilentlyContinue
    
    $mapping = Get-Content -Path "$PSScriptRoot\mapping_v2.json" -Raw
    Invoke-RestMethod -Uri "http://localhost:9200/the_monkeys_blogs_v2" -Method Put -Body $mapping -ContentType "application/json"
} catch {
    Write-Host "Error creating v2 index: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# 2. Reindex
Write-Host "Reindexing data..."
try {
    $reindex = Get-Content -Path "$PSScriptRoot\reindex.json" -Raw
    $res = Invoke-RestMethod -Uri "http://localhost:9200/_reindex?wait_for_completion=true" -Method Post -Body $reindex -ContentType "application/json"
    if ($res.failures.Count -gt 0) {
        Write-Host "Reindex reported failures!" -ForegroundColor Red
        Write-Host ($res.failures | ConvertTo-Json -Depth 5)
        # Continue? Maybe not.
    } else {
        Write-Host "Reindex successful." -ForegroundColor Green
    }
} catch {
    Write-Host "Error reindexing: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# 3. Swap Alias
Write-Host "Swapping Alias..."
try {
    # Delete old concrete index if it exists (which it does after snapshot restore)
    # Check if 'the_monkeys_blogs' is an index or alias
    $isAlias = $false
    try {
        $aliases = Invoke-RestMethod -Uri "http://localhost:9200/_alias/the_monkeys_blogs" -Method Get -ErrorAction Stop
        if ($aliases.Keys -contains "the_monkeys_blogs_v2") { $isAlias = $true }
    } catch {
        # 404 means neither index nor alias, or just index without alias? 
        # _alias/name returns { "indexname": { "aliases": { "name": {} } } }
    }

    if (-not $isAlias) {
        Write-Host "Deleting old concrete index 'the_monkeys_blogs'..."
        Invoke-RestMethod -Uri "http://localhost:9200/the_monkeys_blogs" -Method Delete
    }

    # Create Alias
    Write-Host "Creating alias 'the_monkeys_blogs' -> 'the_monkeys_blogs_v2'..."
    $aliasBody = @{
        actions = @(
            @{ add = @{ index = "the_monkeys_blogs_v2"; alias = "the_monkeys_blogs" } }
        )
    } | ConvertTo-Json -Depth 5
    
    Invoke-RestMethod -Uri "http://localhost:9200/_aliases" -Method Post -Body $aliasBody -ContentType "application/json"
    
} catch {
    Write-Host "Error swapping alias: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "Migration Complete!" -ForegroundColor Green
