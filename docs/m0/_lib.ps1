# M0 共用函数。用法：. "$PSScriptRoot\_lib.ps1"
# 环境：temp/m0-env（WSL docker：new-api:3000，mock:9100）。token 从 temp/m0-env/token.txt 读取，不进文档。
$script:Root   = Resolve-Path "$PSScriptRoot\..\.."
$script:Env    = Join-Path $Root 'temp\m0-env'
$script:Base   = 'http://localhost:3000'
$script:Mock   = 'http://localhost:9100'      # 宿主机访问 mock
$script:MockIn = 'http://mock:9100'           # new-api 容器内访问 mock
$script:Tok    = (Get-Content (Join-Path $Env 'token.txt') -Raw).Trim()
$script:Adm    = @{ Authorization = $Tok; 'New-Api-User' = '1' }

function Api([string]$Method, [string]$Path, $Body = $null) {
    $p = @{ Method = $Method; Uri = "$Base$Path"; Headers = $Adm; ContentType = 'application/json; charset=utf-8'; UseBasicParsing = $true }
    if ($null -ne $Body) { $p.Body = [Text.Encoding]::UTF8.GetBytes(($Body | ConvertTo-Json -Depth 10 -Compress)) }
    try { $r = Invoke-WebRequest @p; $r.Content | ConvertFrom-Json } catch {
        if ($_.Exception.Response) { $sr = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); $sr.ReadToEnd() | ConvertFrom-Json } else { throw }
    }
}
function SetOpt([string]$Key, $Value) { Api PUT '/api/option/' @{ key = $Key; value = $Value } }
function GetOpt([string]$Key) { ((Api GET '/api/option/').data | Where-Object key -eq $Key).value }
function Ctl($Body) { Invoke-RestMethod -Method Post -Uri "$Mock/_ctl" -Body ($Body | ConvertTo-Json -Compress) -ContentType 'application/json' }
function Calls { ,@((Invoke-WebRequest -UseBasicParsing "$Mock/_calls").Content | ConvertFrom-Json) }
function ClearCalls { Invoke-WebRequest -UseBasicParsing -Method Delete "$Mock/_calls" | Out-Null }
function NewChannel($ch) { Api POST '/api/channel/' @{ mode = 'single'; channel = $ch } }
function FindChannel([string]$Name) { (Api GET "/api/channel/search?keyword=$Name&id_sort=true").data }
# 调 relay（用 curl.exe，PS5.1 的 Invoke-WebRequest 读不到错误响应体）。返回 "状态码 正文"
function Relay([string]$Path, [string[]]$Headers, $Body, [int]$Timeout = 60) {
    $f = Join-Path $Env 'req.json'
    [IO.File]::WriteAllText($f, ($Body | ConvertTo-Json -Depth 10 -Compress), (New-Object Text.UTF8Encoding($false)))
    $a = @('-s','-m',$Timeout,'-w',"`n[HTTP %{http_code} ttfb=%{time_starttransfer}s total=%{time_total}s]",'-H','Content-Type: application/json','--data-binary',"@$f")
    foreach ($h in $Headers) { $a += @('-H',$h) }
    & curl.exe @a "$Base$Path"
}
function Tail([int]$n = 5) { (Api GET "/api/log/?p=1&page_size=$n").data }

