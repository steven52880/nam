# M0 ⑫：厂商分组 anthropic + auto 令牌；验证落点、日志 group、按 GroupRatio 扣费
. "$PSScriptRoot\_lib.ps1"
$sk = (Get-Content "$Env\sk_default.txt").Trim()
# 1. 读-改-写三个选项（保留原有项）
$gr = (GetOpt 'GroupRatio') | ConvertFrom-Json; $gr | Add-Member -Force anthropic 0.05
"GroupRatio  => " + (SetOpt 'GroupRatio' ($gr | ConvertTo-Json -Compress) | ConvertTo-Json -Compress)
"AutoGroups  => " + (SetOpt 'AutoGroups' '["anthropic","default"]' | ConvertTo-Json -Compress)
$ug = (GetOpt 'UserUsableGroups') | ConvertFrom-Json; $ug | Add-Member -Force anthropic 'anthropic vendor'
# 发现：令牌 group=auto 时，middleware/auth.go:393 要求 auto 本身也在 UserUsableGroups 里，否则 403 无权访问 auto 分组
$ug | Add-Member -Force auto 'auto'
"UserUsableGroups => " + (SetOpt 'UserUsableGroups' ($ug | ConvertTo-Json -Compress) | ConvertTo-Json -Compress)
"读回: GroupRatio=" + (GetOpt 'GroupRatio') + "  AutoGroups=" + (GetOpt 'AutoGroups')
# 2. 厂商渠道（group=anthropic，key=ANTH 便于在 /_calls 里区分）
$n = 'nam-m0-anth-' + [guid]::NewGuid().ToString('N').Substring(0,6)
NewChannel @{name=$n; type=58; key='upstream-key-ANTH'; base_url=$MockIn; models='test-model'; group='anthropic'; priority=0; weight=1; status=1; test_model='test-model'} | Out-Null
$ida = (FindChannel $n).items[0].id; "anthropic 渠道 id=$ida"
$n | Set-Content "$Env\ch_anth_name.txt"
# 3. auto 令牌
Api POST '/api/token/' @{name='m0-tok-auto'; remain_quota=0; unlimited_quota=$true; expired_time=-1; model_limits_enabled=$false; group='auto'} | ConvertTo-Json -Compress
$tk = (Api GET '/api/token/?p=1&page_size=10').data.items | ? name -eq 'm0-tok-auto'
$key = (Api POST "/api/token/$($tk.id)/key").data.key
"令牌 group=" + $tk.group
Set-Content -Encoding ascii "$Env\sk_auto.txt" "sk-$key"
# 4. 请求
ClearCalls
"--- test-model via auto"
Relay '/v1/chat/completions' @("Authorization: Bearer sk-$key") @{model='test-model';messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
"--- gpt-4o via auto（只有 default 分组的 ch1 有此模型 -> 应回落 default）"
Relay '/v1/chat/completions' @("Authorization: Bearer sk-$key") @{model='gpt-4o';messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
Start-Sleep 1
"--- /_calls"
(Calls) | % { $_ } | Select-Object path,auth_header,model | Format-Table | Out-String
"--- 日志"
(Api GET '/api/log/?p=1&page_size=2&type=2').data.items | % { $o = $_.other | ConvertFrom-Json; "model=$($_.model_name) channel=$($_.channel) group=$($_.group) quota=$($_.quota) pt=$($_.prompt_tokens) ct=$($_.completion_tokens) model_ratio=$($o.model_ratio) completion_ratio=$($o.completion_ratio) group_ratio=$($o.group_ratio) cache_ratio=$($o.cache_ratio)" }
"--- 预期 test-model: (1000*1.5 + 500*1.5*5) * 0.05 = $((1000*1.5 + 500*1.5*5) * 0.05)"

