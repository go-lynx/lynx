# Snowflake ID plugin test script
# Run all unit tests and integration tests

Write-Host "=== Snowflake ID Plugin Tests ===" -ForegroundColor Green

# Setup test environment
$ErrorActionPreference = "Stop"
$TestDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Check if Redis is available
Write-Host "Checking Redis connection..." -ForegroundColor Yellow
try {
    $redisTest = redis-cli -h localhost -p 6379 ping 2>$null
    if ($redisTest -ne "PONG") {
        Write-Host "Warning: Redis service unavailable, skipping integration tests" -ForegroundColor Yellow
        $SkipIntegration = $true
    } else {
        Write-Host "Redis connection OK" -ForegroundColor Green
        $SkipIntegration = $false
    }
} catch {
    Write-Host "Warning: Unable to connect to Redis, skipping integration tests" -ForegroundColor Yellow
    $SkipIntegration = $true
}

# Enter plugin directory
Set-Location $TestDir

# Run unit tests
Write-Host "`n=== Running Unit Tests ===" -ForegroundColor Cyan

Write-Host "Testing Snowflake ID generator..." -ForegroundColor White
go test -v -run "TestSnowflakeGenerator" -timeout 30s

Write-Host "`nTesting WorkerID manager..." -ForegroundColor White
go test -v -run "TestWorkerIDManager" -timeout 30s

Write-Host "`nTesting plugin interface..." -ForegroundColor White
go test -v -run "TestPlugin" -timeout 30s

# Run integration tests (if Redis is available)
if (-not $SkipIntegration) {
    Write-Host "`n=== Running Integration Tests ===" -ForegroundColor Cyan
    
    Write-Host "Cleaning Redis test data..." -ForegroundColor White
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
    
    Write-Host "Running integration tests..." -ForegroundColor White
    go test -v -run "TestRunIntegrationTestSuite" -timeout 60s
    
    Write-Host "Cleaning Redis test data..." -ForegroundColor White
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
}

# Run performance tests
Write-Host "`n=== Running Performance Tests ===" -ForegroundColor Cyan

Write-Host "Generator performance test..." -ForegroundColor White
go test -v -run "^$" -bench "BenchmarkSnowflakeGenerator" -benchtime 3s

if (-not $SkipIntegration) {
    Write-Host "`nIntegration performance test..." -ForegroundColor White
    go test -v -run "^$" -bench "BenchmarkIntegration" -benchtime 3s
    
    # Cleanup
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
}

# Run race condition detection
Write-Host "`n=== Running Race Condition Detection ===" -ForegroundColor Cyan
go test -race -v -run "TestSnowflakeGenerator_ConcurrentGeneration" -timeout 30s

if (-not $SkipIntegration) {
    go test -race -v -run "TestConcurrentGeneration" -timeout 60s
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
}

# Generate test coverage report
Write-Host "`n=== Generating Test Coverage Report ===" -ForegroundColor Cyan
go test -coverprofile=coverage.out -covermode=atomic ./...
if (Test-Path "coverage.out") {
    go tool cover -html=coverage.out -o coverage.html
    Write-Host "Coverage report generated: coverage.html" -ForegroundColor Green
    
    # Display coverage statistics
    $coverage = go tool cover -func=coverage.out | Select-String "total:"
    Write-Host "Total coverage: $($coverage -replace '.*total:\s+\(statements\)\s+', '')" -ForegroundColor Green
}

Write-Host "`n=== Tests Completed ===" -ForegroundColor Green

# Check if any tests failed
if ($LASTEXITCODE -ne 0) {
    Write-Host "Some tests failed, please check the output" -ForegroundColor Red
    exit 1
} else {
    Write-Host "All tests passed!" -ForegroundColor Green
    exit 0
}