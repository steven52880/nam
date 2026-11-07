# M0 ④：不写任何价格配置，常用模型走一遍，对比 NewAPI 默认生效价格与 LiteLLM 官方价
. "$PSScriptRoot\_lib.ps1"
$sk = (Get-Content "$Env\sk_default.txt").Trim()
$name = (Get-Content "$Env\ch1name.txt").Trim(); $id = (FindChannel $name).items[0].id
$models = 'claude-sonnet-4-5-20250929','claude-sonnet-4-5','claude-opus-4-1-20250805','claude-haiku-4-5-20251001','claude-opus-4-5-20251101','gpt-4o','gpt-4.1-mini','gpt-5','gemini-2.5-pro','gemini-2.5-flash','deepseek-chat','o3'
Api PUT '/api/channel/' @{id=$id; models=(($models + 'test-model') -join ',')} | Out-Null
$lit = Get-Content "$Env\litellm.json" -Raw -Encoding UTF8 | ConvertFrom-Json
$litKey = @{'claude-sonnet-4-5-20250929'='claude-sonnet-4-5-20250929';'claude-opus-4-5-20251101'='claude-opus-4-5-20251101';'claude-haiku-4-5-20251001'='claude-haiku-4-5-20251001';'gpt-4o'='gpt-4o';'gpt-4.1-mini'='gpt-4.1-mini';'gpt-5'='gpt-5';'gemini-2.5-pro'='gemini/gemini-2.5-pro';'gemini-2.5-flash'='gemini/gemini-2.5-flash';'deepseek-chat'='deepseek/deepseek-chat';'o3'='o3'}
foreach ($m in $models) {
  $isClaude = $m -like 'claude*'
  if ($isClaude) { $r = Relay '/v1/messages' @("x-api-key: $sk","anthropic-version: 2023-06-01") @{model=$m;max_tokens=16;messages=@(@{role='user';content='hi'})} }
  else { $r = Relay '/v1/chat/completions' @("Authorization: Bearer $sk") @{model=$m;messages=@(@{role='user';content='hi'})} }
  $code = ($r | Select-Object -Last 1)
  if ($code -notmatch 'HTTP 200') { "$m => $code  $(($r | Select-Object -First 1).Substring(0,[Math]::Min(120,($r | Select-Object -First 1).Length)))"; continue }
}
Start-Sleep 1
$items = (Api GET '/api/log/?p=1&page_size=50&type=2').data.items | Sort-Object id
"model | quota | model_ratio | completion_ratio | cache_ratio | create_cache_ratio | NewAPI_USD | Official_USD(LiteLLM) | diff%"
foreach ($it in $items) {
  $o = $it.other | ConvertFrom-Json
  $usd = $it.quota / 500000
  $k = $litKey[$it.model_name]; $off = ''; $diff = ''
  if ($k -and $lit.$k) {
    $L = $lit.$k
    if ($it.model_name -like 'claude*') { $in=800; $cr=200; $cw=0; $out=500 } else { $in=800; $cr=200; $cw=0; $out=500 }  # 两种 mock 用量折算后均为 非缓存800/缓存读200/输出500
    $crp = if ($null -ne $L.cache_read_input_token_cost) { $L.cache_read_input_token_cost } else { $L.input_cost_per_token }
    $off = $in*$L.input_cost_per_token + $cr*$crp + $out*$L.output_cost_per_token
    $diff = [math]::Round(($usd/$off - 1)*100, 2)
    $off = [math]::Round($off, 6)
  }
  "$($it.model_name) | $($it.quota) | $($o.model_ratio) | $($o.completion_ratio) | $($o.cache_ratio) | $($o.cache_creation_ratio) | $([math]::Round($usd,6)) | $off | $diff"
}
