# 生成国旗头像

## 测试

- 1.启动服务

```bash
docker-compose build

docker-compose up -d
```

- 2.浏览器访问测试

http://127.0.0.1:8000/render/base64

http://127.0.0.1:8000/render/img

- 3.停止服务

```bash
docker-compose down --remove-orphans -v
```
