# M0 ⑥：非流式 vs 流式的 frt；流式用 curl 看首字节时间对照
. "$PSScriptRoot\_lib.ps1"
$sk = (Get-Content "$Env\sk_default.txt").Trim()
"--- 非流式 chat"
Relay '/v1/chat/completions' @("Authorization: Bearer $sk") @{model='test-model';messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- 流式 chat"
Relay '/v1/chat/completions' @("Authorization: Bearer $sk") @{model='test-model';stream=$true;stream_options=@{include_usage=$true};messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- 非流式 messages"
Relay '/v1/messages' @("x-api-key: $sk","anthropic-version: 2023-06-01") @{model='test-model';max_tokens=16;messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- 流式 messages"
Relay '/v1/messages' @("x-api-key: $sk","anthropic-version: 2023-06-01") @{model='test-model';max_tokens=16;stream=$true;messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- 流式 generateContent"
Relay '/v1beta/models/test-model:streamGenerateContent?alt=sse' @("x-goog-api-key: $sk") @{contents=@(@{role='user';parts=@(@{text='hi'})})} | Select-Object -Last 1
Start-Sleep 1
"--- 最近 5 条日志"
(Api GET '/api/log/?p=1&page_size=5&type=2').data.items | % { $o = $_.other | ConvertFrom-Json; "id=$($_.id) path=$($o.request_path) stream=$($_.is_stream) use_time=$($_.use_time)s frt=$($o.frt) pt=$($_.prompt_tokens) ct=$($_.completion_tokens) quota=$($_.quota)" }
