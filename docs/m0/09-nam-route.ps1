# M0 ⑨：待嵌入镜像完成后验证（需 patched 镜像注册 GET /nam/ping 与 /api/nam/ping）。
# 待测：访问 /nam/ping、/nam/xxx（SPA 深链）、/api/nam/ping，确认 static.Serve('/') 与 NoRoute 不拦截。
Write-Host '待嵌入镜像完成后验证'
