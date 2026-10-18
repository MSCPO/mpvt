# mpvt (MSCPO ProjectVitals Tool)

> 类 Git 设计的开发者进度与生命体征上报 CLI 工具。面向闭源/开源协同体系，提交“做了什么”的文本修改记录（无代码提交），实现项目透明化与工作量追溯。

---

## 🌟 核心特性

1. **无代码修改记录 (`mpvt commit`)**：
   - 开发者记录工作项（如“优化网络连接池，吞吐提升 35%”），不包含任何源码或商业机密。
   - 解决闭源协作中“进度不透明、不知道谁做了什么”的管理痛点。
2. **Git 仓库深度联动 (`--from-git`)**：
   - 自动提取本地仓库最近一次 `git commit` 的标题与说明，一键同步上报至 ProjectVitals。
3. **工作区状态机管理 (`mpvt status`)**：
   - 识别当前项目所属生命周期状态（`IN_DEVELOPMENT` 开发中 / `PUBLISHED` 已发布 / `PLANNING` 理论构想中 / `ARCHIVED` 归档锁死）。
4. **里程碑与路线图绑定 (`mpvt milestone`)**：
   - 切换当前工作区的默认里程碑（`mpvt milestone use <id>`），后续 commit 自动归档关联。
5. **构想修改提案与段落追溯 (`mpvt pr` & `mpvt blame`)**：
   - 在 `PLANNING` 状态下支持类似 GitHub PR 的修改提案流与段落作者溯源。

---

## 🚀 快速上手

### 1. 编译安装
```bash
# 进入工程目录
cd /media/hokkotaru/534205DE4152/mscpo-projectvitals-cli

# 编译单二进制文件
go build -o mpvt .

# （可选）移动至 PATH 目录
sudo cp mpvt /usr/local/bin/
```

### 2. 账号登录认证 (`mpvt login`)
```bash
# 执行登录：CLI 将自动启动本地服务并唤起浏览器展示带 hCaptcha 人机验证的登录页面
mpvt login

# 登录完成后，凭据将自动同步并保存至 ~/.config/mpvt/config.yaml

# （可选）也可以在无浏览器/CI环境中直接传入已有 Token：
mpvt login --token <YOUR_JWT_TOKEN>

# 查看当前连接状态与登录身份
mpvt auth status
```

### 3. 在代码仓库中绑定项目 (`mpvt init` / `mpvt link`)
进入您的开发仓库目录：
```bash
cd /path/to/your/project

# 交互式选择项目，或直接指定 slug
mpvt link vitals-core
```
> CLI 会在当前目录下创建 `.mpvt/config.json`，并自动将 `.mpvt/` 添加至 `.gitignore`。

### 4. 提交工作记录 (`mpvt commit`)
```bash
# 方式 A：命令行直接指定
mpvt commit -m "优化网络连接池，吞吐量提升35%" -d "调整了超时重试机制，消除了长连接空闲泄漏问题"

# 方式 B：从本地 Git 提交自动提取
mpvt commit --from-git

# 方式 C：无参数交互式表单录入
mpvt commit
```

### 5. 查看进度时间线 (`mpvt log`)
```bash
# 查看完整时间线
mpvt log

# 单行紧凑显示
mpvt log --oneline -n 20
```

---

## 📖 命令完整参考

| 命令 | 别名 | 说明 |
| :--- | :--- | :--- |
| `mpvt auth login` | - | 配置后端地址与认证 Token |
| `mpvt auth status` | `whoami` | 查看当前鉴权状态与服务器健康度 |
| `mpvt auth logout` | - | 清除已保存的认证凭据 |
| `mpvt init [slug]` | `link` | 关联当前目录与远端项目 |
| `mpvt unlink` | - | 解除当前目录与远端项目的绑定 |
| `mpvt status` | - | 查看当前项目生命周期、默认里程碑及最近动态 |
| `mpvt commit` | - | 提交无代码修改记录 |
| `mpvt log` | `history` | 浏览进度提交历史时间线 |
| `mpvt milestone list` | `ms ls` | 列出当前项目里程碑与路线图 |
| `mpvt milestone use <id>` | - | 切换当前工作区的默认里程碑 |
| `mpvt milestone create` | - | 新增里程碑/路线图节点 |
| `mpvt milestone bind <recId>` | - | 动态关联或解绑记录到里程碑 |
| `mpvt pr list` | `proposal ls` | 浏览规划期（PLANNING）修改提案 |
| `mpvt pr create` | - | 提交理论构想修改提案 |
| `mpvt pr merge <id>` | - | 合并修改提案（需项目管理员权限） |
| `mpvt blame` | `concept` | 查看构想文档各段落的作者贡献追溯 |
| `mpvt new [title]` | `create`, `project create` | **创建新项目**（支持交互式向导与自动绑定当前目录） |
| `mpvt project list` | `proj ls` | 浏览生态项目列表 |
| `mpvt project view [slug]` | - | 查看项目详细信息及成员阵容 |
| `mpvt version` | - | 查看 CLI 版本信息 |

---

## ⚙️ 环境变量支持

| 环境变量 | 作用 |
| :--- | :--- |
| `MPVT_ENDPOINT` | 覆盖 API 端点（默认 `https://api.mscpo.com/api/projectvitals`） |
| `MPVT_TOKEN` | 覆盖 Bearer JWT 认证令牌 |
| `MPVT_INTERNAL_TOKEN` | 微服务内部网关通信签名（开发/网关调试模式） |
| `MPVT_USER_ID` | 网关伪装用户 ID |
| `MPVT_USER_EMAIL` | 网关伪装用户邮箱 |
| `MPVT_USER_ROLE` | 网关伪装用户角色（`USER` / `ADMIN`） |

---

## 🤖 CI/CD 自动化上报示例 (GitHub Actions)

当分支合并至 `main` 时，自动生成一条 ProjectVitals 记录：

```yaml
name: ProjectVitals Reporter
on:
  push:
    branches: [main]

jobs:
  report:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install mpvt
        run: |
          curl -sSL https://releases.mscpo.org/mpvt/install.sh | bash
      - name: Report Commit
        env:
          MPVT_ENDPOINT: ${{ secrets.MPVT_ENDPOINT }}
          MPVT_TOKEN: ${{ secrets.MPVT_TOKEN }}
        run: |
          mpvt --project my-project-slug commit --from-git
```
