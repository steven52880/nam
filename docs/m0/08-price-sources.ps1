# M0 ⑧：LiteLLM vs models.dev 字段映射。样本已下载到 temp/m0-env/{litellm,modelsdev}.json
# 下载命令：curl.exe -x http://127.0.0.1:7890 -L -o litellm.json https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json
#           curl.exe -x http://127.0.0.1:7890 -L -o modelsdev.json https://models.dev/api.json
. "$PSScriptRoot\_lib.ps1"
$l = Get-Content "$Env\litellm.json" -Raw -Encoding UTF8 | ConvertFrom-Json
$m = Get-Content "$Env\modelsdev.json" -Raw -Encoding UTF8 | ConvertFrom-Json
# 模型名 -> (LiteLLM key, models.dev provider, models.dev id)
$list = @(
 @('claude-sonnet-4-5','claude-sonnet-4-5-20250929','anthropic','claude-sonnet-4-5'),
 @('claude-opus-4-1','claude-opus-4-1-20250805','anthropic','claude-opus-4-1'),
 @('claude-opus-4-5','claude-opus-4-5-20251101','anthropic','claude-opus-4-5'),
 @('claude-haiku-4-5','claude-haiku-4-5-20251001','anthropic','claude-haiku-4-5'),
 @('claude-sonnet-4','claude-sonnet-4-20250514','anthropic','claude-sonnet-4-0'),
 @('claude-3-5-haiku','claude-3-5-haiku-20241022','anthropic','claude-3-5-haiku-latest'),
 @('gpt-4o','gpt-4o','openai','gpt-4o'),
 @('gpt-4o-mini','gpt-4o-mini','openai','gpt-4o-mini'),
 @('gpt-4.1','gpt-4.1','openai','gpt-4.1'),
 @('gpt-4.1-mini','gpt-4.1-mini','openai','gpt-4.1-mini'),
 @('gpt-4.1-nano','gpt-4.1-nano','openai','gpt-4.1-nano'),
 @('gpt-5','gpt-5','openai','gpt-5'),
 @('gpt-5-mini','gpt-5-mini','openai','gpt-5-mini'),
 @('o3','o3','openai','o3'),
 @('o4-mini','o4-mini','openai','o4-mini'),
 @('gemini-2.5-pro','gemini/gemini-2.5-pro','google','gemini-2.5-pro'),
 @('gemini-2.5-flash','gemini/gemini-2.5-flash','google','gemini-2.5-flash'),
 @('gemini-2.5-flash-lite','gemini/gemini-2.5-flash-lite','google','gemini-2.5-flash-lite'),
 @('deepseek-chat','deepseek/deepseek-chat','deepseek','deepseek-chat'),
 @('deepseek-reasoner','deepseek/deepseek-reasoner','deepseek','deepseek-reasoner')
)
function M($v) { if ($null -eq $v) { '-' } else { [math]::Round([double]$v * 1e6, 4) } }
$rows = foreach ($x in $list) {
  $a = $l.($x[1]); $b = $m.($x[2]).models.($x[3])
  [pscustomobject]@{
    model=$x[0]
    L_in=if($a){M $a.input_cost_per_token}else{'MISS'}; L_out=if($a){M $a.output_cost_per_token}else{'MISS'}
    L_cr=if($a){M $a.cache_read_input_token_cost}else{'MISS'}; L_cw5=if($a){M $a.cache_creation_input_token_cost}else{'MISS'}; L_cw1h=if($a){M $a.cache_creation_input_token_cost_above_1hr}else{'MISS'}
    D_in=if($b){$b.cost.input}else{'MISS'}; D_out=if($b){$b.cost.output}else{'MISS'}
    D_cr=if($b){if($null -ne $b.cost.cache_read){$b.cost.cache_read}else{'-'}}else{'MISS'}; D_cw=if($b){if($null -ne $b.cost.cache_write){$b.cost.cache_write}else{'-'}}else{'MISS'}
  }
}
$rows | Format-Table -AutoSize | Out-String -Width 300
"--- LiteLLM 字段全集中与价格相关的 key（抽样 anthropic 与 openai 条目）"
($l.'claude-sonnet-4-5-20250929'.PSObject.Properties.Name | ? { $_ -match 'cost|price' }) -join ', '
($l.'gpt-4o'.PSObject.Properties.Name | ? { $_ -match 'cost|price' }) -join ', '
"--- LiteLLM 带 '/' 前缀的 key 数 / 总数: $(($l.PSObject.Properties.Name | ? { $_ -match '/' }).Count) / $(($l.PSObject.Properties).Count)"
"--- models.dev cost 子字段全集"
$set=@{}; foreach ($p in $m.PSObject.Properties) { if ($p.Value.models) { foreach ($mm in $p.Value.models.PSObject.Properties) { if ($mm.Value.cost) { foreach ($k in $mm.Value.cost.PSObject.Properties.Name) { $set[$k]=1+[int]$set[$k] } } } } }
($set.GetEnumerator() | Sort-Object Value -Descending | % { "$($_.Key)=$($_.Value)" }) -join ', '
