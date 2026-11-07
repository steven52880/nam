# M0 ⑤：关全量自动测试 + 开自动禁用/自动恢复；401 触发禁用；恢复后等待一个周期。耗时约 2–3 分钟
. "$PSScriptRoot\_lib.ps1"
$sk = (Get-Content "$Env\sk_default.txt").Trim()
$n1 = (Get-Content "$Env\ch1name.txt").Trim(); $id1 = (FindChannel $n1).items[0].id
$n2 = 'nam-m0-ch2-' + [guid]::NewGuid().ToString('N').Substring(0,6)
NewChannel @{name=$n2; type=58; key='upstream-key-B'; base_url=$MockIn; models='test-model'; group='default'; priority=1; weight=1; status=1; test_model='test-model'} | Out-Null
$id2 = (FindChannel $n2).items[0].id
"ch1(id=$id1,key=A,prio=3) ch2(id=$id2,key=B,prio=1)"
function St { "ch$id1=" + (Api GET "/api/channel/$id1").data.status + " ch$id2=" + (Api GET "/api/channel/$id2").data.status }
SetOpt 'AutomaticDisableChannelEnabled' 'true' | Out-Null
SetOpt 'AutomaticEnableChannelEnabled' 'true' | Out-Null
SetOpt 'monitor_setting.auto_test_channel_enabled' 'false' | Out-Null
SetOpt 'monitor_setting.auto_test_channel_minutes' '1' | Out-Null
"初始: " + (St)
Ctl @{reset=$true} | Out-Null
Ctl @{fail_next=1; fail_status=401} | Out-Null
"--- 发一次请求，mock 先返回 401（落到 ch1），重试可能转 ch2"
Relay '/v1/chat/completions' @("Authorization: Bearer $sk") @{model='test-model';messages=@(@{role='user';content='hi'})} | Select-Object -Last 1
Start-Sleep 2
"请求后: " + (St)
"--- 请求阶段 /_calls"
(Calls) | % { $_ } | Select-Object time,path,auth_header,status | Format-Table | Out-String
Ctl @{reset=$true} | Out-Null     # mock 恢复正常并清空 calls
$t0 = Get-Date; "恢复 mock 于 $($t0.ToString('HH:mm:ss'))，开始每 5s 轮询"
$recovered = $null
for ($i=0; $i -lt 48; $i++) {
  Start-Sleep 5
  $s = (Api GET "/api/channel/$id1").data.status
  if ($s -eq 1) { $recovered = [int]((Get-Date) - $t0).TotalSeconds; break }
}
"ch1 恢复耗时: $recovered 秒（null=超时未恢复）"
"最终: " + (St)
"--- 恢复阶段 /_calls（auth_header 的 key：A=ch1，B=ch2）"
(Calls) | % { $_ } | Select-Object @{n='t';e={[DateTimeOffset]::FromUnixTimeMilliseconds($_.time).LocalDateTime.ToString('HH:mm:ss')}},path,auth_header,x_api_key,status | Format-Table | Out-String
"--- 清理 ch2 与选项"
Api DELETE "/api/channel/$id2" | Out-Null
SetOpt 'AutomaticEnableChannelEnabled' 'false' | Out-Null
SetOpt 'AutomaticDisableChannelEnabled' 'false' | Out-Null
