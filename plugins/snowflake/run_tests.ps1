# 雪花ID插件测试脚本
# 运行所有单元测试和集成测试

Write-Host "=== 雪花ID插件测试 ===" -ForegroundColor Green

# 设置测试环境
$ErrorActionPreference = "Stop"
$TestDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# 检查Redis是否可用
Write-Host "检查Redis连接..." -ForegroundColor Yellow
try {
    $redisTest = redis-cli -h localhost -p 6379 ping 2>$null
    if ($redisTest -ne "PONG") {
        Write-Host "警告: Redis服务不可用，将跳过集成测试" -ForegroundColor Yellow
        $SkipIntegration = $true
    } else {
        Write-Host "Redis连接正常" -ForegroundColor Green
        $SkipIntegration = $false
    }
} catch {
    Write-Host "警告: 无法连接Redis，将跳过集成测试" -ForegroundColor Yellow
    $SkipIntegration = $true
}

# 进入插件目录
Set-Location $TestDir

# 运行单元测试
Write-Host "`n=== 运行单元测试 ===" -ForegroundColor Cyan

Write-Host "测试雪花ID生成器..." -ForegroundColor White
go test -v -run "TestSnowflakeGenerator" -timeout 30s

Write-Host "`n测试WorkerID管理器..." -ForegroundColor White
go test -v -run "TestWorkerIDManager" -timeout 30s

Write-Host "`n测试插件接口..." -ForegroundColor White
go test -v -run "TestPlugin" -timeout 30s

# 运行集成测试（如果Redis可用）
if (-not $SkipIntegration) {
    Write-Host "`n=== 运行集成测试 ===" -ForegroundColor Cyan
    
    Write-Host "清理Redis测试数据..." -ForegroundColor White
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
    
    Write-Host "运行集成测试..." -ForegroundColor White
    go test -v -run "TestRunIntegrationTestSuite" -timeout 60s
    
    Write-Host "清理Redis测试数据..." -ForegroundColor White
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
}

# 运行性能测试
Write-Host "`n=== 运行性能测试 ===" -ForegroundColor Cyan

Write-Host "生成器性能测试..." -ForegroundColor White
go test -v -run "^$" -bench "BenchmarkSnowflakeGenerator" -benchtime 3s

if (-not $SkipIntegration) {
    Write-Host "`n集成性能测试..." -ForegroundColor White
    go test -v -run "^$" -bench "BenchmarkIntegration" -benchtime 3s
    
    # 清理
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
}

# 运行竞态条件检测
Write-Host "`n=== 运行竞态条件检测 ===" -ForegroundColor Cyan
go test -race -v -run "TestSnowflakeGenerator_ConcurrentGeneration" -timeout 30s

if (-not $SkipIntegration) {
    go test -race -v -run "TestConcurrentGeneration" -timeout 60s
    redis-cli -h localhost -p 6379 -n 15 FLUSHDB > $null
}

# 生成测试覆盖率报告
Write-Host "`n=== 生成测试覆盖率报告 ===" -ForegroundColor Cyan
go test -coverprofile=coverage.out -covermode=atomic ./...
if (Test-Path "coverage.out") {
    go tool cover -html=coverage.out -o coverage.html
    Write-Host "覆盖率报告已生成: coverage.html" -ForegroundColor Green
    
    # 显示覆盖率统计
    $coverage = go tool cover -func=coverage.out | Select-String "total:"
    Write-Host "总覆盖率: $($coverage -replace '.*total:\s+\(statements\)\s+', '')" -ForegroundColor Green
}

Write-Host "`n=== 测试完成 ===" -ForegroundColor Green

# 检查是否有测试失败
if ($LASTEXITCODE -ne 0) {
    Write-Host "部分测试失败，请检查输出" -ForegroundColor Red
    exit 1
} else {
    Write-Host "所有测试通过！" -ForegroundColor Green
    exit 0
}