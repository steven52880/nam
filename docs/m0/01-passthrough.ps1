# M0 ①：type=58 PassThrough 渠道透传 chat / messages / generateContent
. "$PSScriptRoot\_lib.ps1"
$sk = (Get-Content "$Env\sk_default.txt").Trim()
# 前置：给 test-model 写价格（仅本测试用）。ModelRatio 需要整表读-改-写
$mr = (GetOpt 'ModelRatio') | ConvertFrom-Json; $mr | Add-Member -Force test-model 1.5 -PassThru | Out-Null
SetOpt 'ModelRatio' ($mr | ConvertTo-Json -Compress) | Out-Null
$cr = (GetOpt 'CompletionRatio') | ConvertFrom-Json; $cr | Add-Member -Force test-model 5 -PassThru | Out-Null
SetOpt 'CompletionRatio' ($cr | ConvertTo-Json -Compress) | Out-Null
ClearCalls
"--- A. chat/completions (Authorization: Bearer sk-...)"
Relay '/v1/chat/completions' @("Authorization: Bearer $sk") @{model='test-model';messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- B. messages (x-api-key: sk-...)"
Relay '/v1/messages' @("x-api-key: $sk","anthropic-version: 2023-06-01") @{model='test-model';max_tokens=16;messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- B2. messages (Authorization: Bearer)"
Relay '/v1/messages' @("Authorization: Bearer $sk","anthropic-version: 2023-06-01") @{model='test-model';max_tokens=16;messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- C. generateContent (x-goog-api-key: sk-...)"
Relay '/v1beta/models/test-model:generateContent' @("x-goog-api-key: $sk") @{contents=@(@{role='user';parts=@(@{text='hi'})})} | Select-Object -Last 1
"--- C2. generateContent (?key=sk-...)"
Relay "/v1beta/models/test-model:generateContent?key=$sk" @() @{contents=@(@{role='user';parts=@(@{text='hi'})})} | Select-Object -Last 1
"--- /_calls"
(Calls) | % { $_ } | Select-Object method,path,auth_header,x_api_key,x_goog_api_key,model,stream,status | Format-Table -AutoSize | Out-String -Width 250

