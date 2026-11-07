# 从调试台启动游戏

启动时指定默认请求：

```sh
cle-debug --config ./launch.json
```

进入终端后输入 `launch`；也可指定请求：`launch --config ./examples/launch/vanilla.json`。

终端会显示启动任务 ID、版本、Java 路径、账户类型、游戏目录、加载器、当前阶段和可用的 PID，并输出事件，例如：

```text
[task_status task=cle-... INFO] Launch task status changed to downloading
[download_progress task=cle-... INFO] Downloaded ... (12/100 files)
[process_started task=cle-... pid=1234 INFO] Minecraft process started
[game_stdout task=cle-... pid=1234 INFO] ...
```

用 `watch <task-id>` 或 `logs --task <task-id>` 筛选输出。`debug arguments <task-id>` 显示脱敏后的解析参数。不要把原始凭据写入启动请求。

## 使用已保存的账户

使用 `account add offline Alex` 创建离线账户，或使用 `account add yggdrasil <auth-server-url> <username>` 添加 Yggdrasil 账户。后者会隐藏密码输入且要求 HTTPS。通过 `account list` 查看后，在启动请求中指定 `{"account":{"id":"<stored-id>"}}`。若请求未指定账户身份，则使用当前活动的已保存账户。内联 `{"account":{"name":"Alex","type":"offline"}}` 格式仍可使用。

Yggdrasil 启动从操作系统凭据库读取会话；Token 不得写入启动 JSON。账户的
Yggdrasil API 基础 URL 会作为 JVM Agent 参数传给 Authlib Injector，使 Minecraft
客户端自身的会话请求也使用第三方服务。首次启动会下载固定版本且经过 SHA-256
校验的 Authlib Injector 1.2.8；离线模式要求游戏目录缓存中已有校验通过的文件。
兼容性取决于 Minecraft/Authlib 版本和服务端；尚未验证真实认证或实机启动。
离线资料只是本地身份，不能证明账户所有权，也不能通过正版在线多人服务器认证。
