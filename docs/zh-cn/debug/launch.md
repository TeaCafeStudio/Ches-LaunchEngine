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

## 使用已保存的离线账户

使用 `account add offline Alex` 创建账户，`account list` 查看。启动请求可包含 `{"account":{"id":"<stored-id>"}}`。若请求未指定账户身份，则使用当前活动离线账户。已有的内联 `{"account":{"name":"Alex","type":"offline"}}` 格式仍可使用。

离线资料只是本地身份，不能证明账户所有权，也不能通过正版在线多人服务器认证。
