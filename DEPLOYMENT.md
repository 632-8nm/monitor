# 閮ㄧ讲鏋舵瀯涓庢惌寤烘祦绋嬶紙orangepi-monitor锛?
> 鏈枃妗ｈ褰曟湰椤圭洰浠庨浂鎼缓鐨勫畬鏁存祦绋嬩笌褰撳墠鐢熶骇鏋舵瀯锛屼緵缁存姢鑰呭弬鑰冦€?> 鏁忔劅淇℃伅涓€寰嬩娇鐢ㄥ崰浣嶇锛屼笉鍖呭惈鐪熷疄 token / 瀵嗙爜銆?
## 1. 鏁翠綋鏋舵瀯

```
寮€鍙戣€呮湰鍦?(Windows)
   鈹? 鍐欎唬鐮?鈫?go vet / go test锛堟湰鍦拌嚜娴嬶級
   鈹? git commit + push
   鈻?GitHub (orangepi-monitor 浠撳簱)
   鈹? 瑙﹀彂 GitHub Actions锛坵orkflow: deploy.yml锛?   鈻?CI/CD锛圙itHub 浜戠 runner锛寀buntu-latest锛?   鈹? 鈶?浜ゅ弶缂栬瘧 arm64 浜岃繘鍒讹紙CGO_ENABLED=0 GOOS=linux GOARCH=arm64锛?   鈹? 鈶?閫氳繃 Cloudflare Tunnel (ssh.<your-domain>) SSH 鍒版澘瀛?   鈻?鏉垮瓙锛圤range Pi Zero 3锛?   鈹? 鍋滄湇鍔?鈫?scp 浜岃繘鍒?+ 鍓嶇 鈫?閲嶅惎
   鈻?/opt/orangepi-monitor/  锛坰ystemd: monitor锛岀洃鍚?:8080锛?```

> 娉細monitor 鐨勫墠绔紙index.html + static/锛夋槸鐙珛鏂囦欢锛堟湭 embed 杩涗簩杩涘埗锛夛紝
> 閮ㄧ讲鏃堕殢浜岃繘鍒朵竴璧蜂紶杈撳埌 /opt/orangepi-monitor/銆?
## 2. 鐢熶骇閮ㄧ讲浣嶇疆

| 椤?| 鍊?|
|---|---|
| 閮ㄧ讲鐩綍 | `/opt/orangepi-monitor/` |
| 浜岃繘鍒?| `/opt/orangepi-monitor/monitor_server` |
| 鍓嶇 | `/opt/orangepi-monitor/index.html` + `static/` |
| systemd 鏈嶅姟 | `monitor.service` |
| 鐩戝惉绔彛 | `8080` |
| 鍏綉鍏ュ彛 | `orangepi-monitor.<your-domain>`锛圕loudflare 闅ч亾 鈫?http://localhost:8080锛?|
| CI 閮ㄧ讲鍏ュ彛 | `ssh.<your-domain>`锛圕loudflare 闅ч亾 鈫?ssh://localhost:22锛?|

## 3. 鏉垮瓙渚х粍浠?
| 缁勪欢 | 璇存槑 | 鐘舵€?|
|---|---|---|
| `cloudflared` | Cloudflare 闅ч亾瀹㈡埛绔紙token 妯″紡锛?| systemd 鏈嶅姟锛屽父椹?|
| `/opt/orangepi-monitor/` | 鐢熶骇閮ㄧ讲鐩綍 | 鐢?CI 鏇存柊 |
| `monitor.service` | systemd 鍗曞厓锛屾寚鍚?`/opt/orangepi-monitor/monitor_server` | 甯搁┗ |

## 4. Cloudflare 渚ч厤缃?
| 椤?| 閰嶇疆 | 鐢ㄩ€?|
|---|---|---|
| 鍩熷悕 | `<your-domain>` | 鎵樼鍦?Cloudflare |
| 闅ч亾 | token 妯″紡锛坱unnel run --token锛?| 鏉垮瓙涓诲姩杩?Cloudflare |
| Public Hostname | `orangepi-monitor.<your-domain>` 鈫?`http://localhost:8080` | 鍏綉璁块棶 Web |
| Public Hostname | `ssh.<your-domain>` 鈫?`ssh://localhost:22` | CI/CD 閮ㄧ讲 SSH 鍏ュ彛锛堜袱椤圭洰鍏辩敤锛?|
| Service Token | 鍦?Access 鈫?Service Auth 鍒涘缓 | CI 璁よ瘉锛堟牸寮?ID:SECRET锛?|
| Access 绛栫暐 | `ci-deploy`锛圫ervice Auth + token锛?| 鏀捐 CI 鐨?cloudflared 杩炴帴 |

## 5. GitHub 渚ч厤缃?
| 椤?| 鍊?| 璇存槑 |
|---|---|---|
| 浠撳簱 | `632-8nm/orangepi-monitor` | 鈥?|
| Workflow | `.github/workflows/deploy.yml` | push main 瑙﹀彂锛屼簯绔紪璇?+ 閮ㄧ讲 |
| Secret: `BOARD_SSH_KEY` | 鏉垮瓙 `~/.ssh/deploy` 绉侀挜 | 浜戠 SSH 鐧诲綍鏉垮瓙锛堜袱椤圭洰鍏辩敤锛?|
| Secret: `CLOUDFLARED_TOKEN` | `ClientID:ClientSecret` | cloudflared access 璁よ瘉锛堜袱椤圭洰鍏辩敤锛?|

## 6. 浠庨浂鎼缓姝ラ

### 6.1 鏉垮瓙鍑嗗
```bash
# 瀹夎 cloudflared
curl -L --output /usr/local/bin/cloudflared \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64
chmod +x /usr/local/bin/cloudflared

# 閰嶇疆鍏嶅瘑 sudo锛堜粎 systemctl/journalctl/tee锛屼緵 CI 浣跨敤锛?sudo tee /etc/sudoers.d/orangepi-systemd <<'EOF'
orangepi ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /bin/systemctl, /usr/bin/journalctl, /usr/bin/tee
EOF
sudo chmod 440 /etc/sudoers.d/orangepi-systemd

# 鐢熸垚 CI 閮ㄧ讲瀵嗛挜
ssh-keygen -t ed25519 -N "" -f ~/.ssh/deploy
cat ~/.ssh/deploy.pub >> ~/.ssh/authorized_keys

# 鍒涘缓鐢熶骇閮ㄧ讲鐩綍
sudo mkdir -p /opt/orangepi-monitor
sudo chown orangepi:orangepi /opt/orangepi-monitor
```

### 6.2 Cloudflare 閰嶇疆
1. 鍩熷悕鎵樼鍦?Cloudflare锛坄<your-domain>`锛?2. 鏉垮瓙瀹夎 cloudflared锛岀敤 token 鎺ュ叆闅ч亾
3. 鍔?Public Hostname锛?   - `orangepi-monitor.<your-domain>` 鈫?`http://localhost:8080`
   - `ssh.<your-domain>` 鈫?`ssh://localhost:22`锛堜袱椤圭洰鍏辩敤锛?4. Access 鈫?Service Auth 鍒涘缓 Service Token锛堣涓?Client ID/Secret锛?5. Access 鈫?涓?`ssh.<your-domain>` 閰嶇疆 `ci-deploy` 绛栫暐锛圫ervice Auth锛?
### 6.3 GitHub 閰嶇疆
1. 浠撳簱鍔犱袱涓?Secret锛堜笌 remote-wakeup 鍏辩敤鍚屼竴浠藉€硷級锛?   - `BOARD_SSH_KEY` = 鏉垮瓙 `~/.ssh/deploy` 绉侀挜鍏ㄦ枃
   - `CLOUDFLARED_TOKEN` = `ClientID:ClientSecret`
2. 鎺ㄩ€?`.github/workflows/deploy.yml`锛坧ush main 鑷姩閮ㄧ讲锛?
### 6.4 楠岃瘉
鎺ㄤ竴娆′唬鐮佸埌 main锛岃瀵?GitHub Actions 鐨?Build & Deploy 鏄惁 success锛?鏉垮瓙 `/opt/orangepi-monitor/monitor_server` 鏄惁鏇存柊銆佹湇鍔℃槸鍚﹂噸鍚€?
## 7. 鏃ュ父缁存姢

```bash
# 鏌ョ湅鏈嶅姟
systemctl status monitor
# 鏌ョ湅鏃ュ織
journalctl -u monitor -f
# 鎵嬪姩閲嶅惎
sudo systemctl restart monitor
# 鏀归厤缃紙鐜鍙橀噺锛?sudo nano /etc/default/monitor && sudo systemctl restart monitor
```

## 8. 鍥炴粴

CI 閮ㄧ讲鐨勬槸浜戠缂栬瘧鐨勫浐瀹氱増鏈簩杩涘埗銆傚洖婊氭柟寮忥細
- 鐢?`git revert` 鍥為€€浠ｇ爜鍚?push锛堣Е鍙戦噸鏂伴儴缃叉棫鐗堬級
- 鎴栨墜鍔ㄦ浛鎹?`/opt/orangepi-monitor/monitor_server` 涓轰笂涓€鐗堜簩杩涘埗骞堕噸鍚?