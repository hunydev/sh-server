# sh.huny.dev - Personal Script Repository

웹 UI로 셸 스크립트를 관리하고, CLI에서 `curl | sh`로 빠르게 실행할 수 있는 서비스.

## 아키텍처

```
┌─────────────────────────────────────────────────────────────┐
│                     sh.huny.dev                              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────┐ │
│  │   Browser   │    │    curl     │    │   search.sh     │ │
│  │   (Admin)   │    │  (Execute)  │    │   (TUI/fzf)     │ │
│  └──────┬──────┘    └──────┬──────┘    └───────┬─────────┘ │
│         │                  │                    │           │
│         ▼                  ▼                    ▼           │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              Content Negotiation Layer                  ││
│  │     (User-Agent / Accept 헤더 기반 응답 분기)            ││
│  └─────────────────────────────────────────────────────────┘│
│         │                  │                    │           │
│         ▼                  ▼                    ▼           │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────┐│
│  │   HTML UI    │   │  text/plain  │   │  Locked Script   ││
│  │   (SPA)      │   │  (Script)    │   │  Auth Flow       ││
│  └──────────────┘   └──────────────┘   └──────────────────┘│
│                             │                    │          │
│                             ▼                    ▼          │
│                    ┌─────────────────────────────────┐      │
│                    │          SQLite DB             │      │
│                    │  (scripts, folders, tokens)    │      │
│                    └─────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## 기능

### CLI 사용법

```bash
# 도움말
curl -fsSL https://sh.huny.dev/help.sh | sh

# TUI 검색 (fzf > whiptail > 숫자 메뉴 폴백)
curl -fsSL https://sh.huny.dev/search.sh | sh

# 특정 스크립트 실행
curl -fsSL https://sh.huny.dev/tools/sysinfo.sh | sh

# 잠금된 스크립트 (암호 입력 프롬프트 자동 표시)
curl -fsSL https://sh.huny.dev/private/secret.sh | sh
```

### 웹 UI

- 폴더 구조 기반 스크립트 관리
- 스크립트 생성/수정/삭제
- 메타데이터 (설명, 태그, 요구사항, 위험도)
- 잠금 설정 (암호 보호)
- 검색 기능

## 데이터 모델

### Scripts 테이블
```sql
CREATE TABLE scripts (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,     -- /tools/check_memory.sh
    name TEXT NOT NULL,            -- check_memory.sh
    content TEXT NOT NULL,
    description TEXT,
    tags TEXT,                     -- comma-separated
    locked INTEGER DEFAULT 0,
    password_hash TEXT,            -- bcrypt
    danger_level INTEGER DEFAULT 0,
    requires TEXT,
    examples TEXT,
    favorite INTEGER DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### Auth Tokens 테이블
```sql
CREATE TABLE auth_tokens (
    token TEXT PRIMARY KEY,
    script_id TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,  -- 5분
    created_at TIMESTAMP,
    ip_address TEXT,
    user_agent TEXT
);
```

## API 엔드포인트

### 공개 엔드포인트

| Method | Path | 설명 |
|--------|------|------|
| GET | / | CLI: 2줄 텍스트, 브라우저: HTML UI |
| GET | /help.sh | 도움말 스크립트 |
| GET | /search.sh | TUI 검색 스크립트 |
| GET | /{path}.sh | 스크립트 내용 (잠금시 암호 프롬프트) |
| GET | /_catalog.json | 스크립트 목록 (메타데이터) |
| POST | /_auth/unlock | 잠금 해제 (토큰 발급) |

### 관리자 API (ADMIN_TOKEN 필요)

| Method | Path | 설명 |
|--------|------|------|
| GET | /api/scripts | 모든 스크립트 목록 |
| POST | /api/scripts | 스크립트 생성 |
| GET | /api/scripts/{id} | 스크립트 조회 |
| PUT | /api/scripts/{id} | 스크립트 수정 |
| DELETE | /api/scripts/{id} | 스크립트 삭제 |
| GET | /api/tree | 폴더 트리 |
| GET | /api/folders | 폴더 목록 |
| POST | /api/folders | 폴더 생성 |
| DELETE | /api/folders/{id} | 폴더 삭제 |
| GET | /api/search?q= | 검색 |

## 잠금 스크립트 플로우

```
┌──────────────┐          ┌──────────────┐          ┌──────────────┐
│   사용자     │          │   서버       │          │   사용자     │
└──────┬───────┘          └──────┬───────┘          └──────┬───────┘
       │                         │                         │
       │  curl /locked.sh        │                         │
       │────────────────────────>│                         │
       │                         │                         │
       │  암호 입력 스크립트     │                         │
       │<────────────────────────│                         │
       │                         │                         │
       │  (실행 후 암호 입력)    │                         │
       │                         │                         │
       │  POST /_auth/unlock     │                         │
       │   {path, password}      │                         │
       │────────────────────────>│                         │
       │                         │  bcrypt 검증            │
       │  {token, expires_at}    │                         │
       │<────────────────────────│                         │
       │                         │                         │
       │  GET /locked.sh?token=  │                         │
       │────────────────────────>│                         │
       │                         │  토큰 검증              │
       │  실제 스크립트 내용     │                         │
       │<────────────────────────│                         │
       │                         │                         │
       │  (스크립트 실행)        │                         │
       ▼                         ▼                         ▼
```

## 환경 변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| PORT | 8000 | 서버 포트 |
| DB_PATH | ./sh.db | SQLite DB 경로 |
| HOSTNAME | sh.huny.dev | 호스트명 (curl 명령어 생성용) |
| ADMIN_TOKEN | (empty) | 관리자 API 토큰 |

## 로컬 실행

```bash
# 빌드
go build -o sh-server ./cmd/srv

# 실행
export ADMIN_TOKEN=your-secret-token
export HOSTNAME=localhost:8000
./sh-server

# 테스트
curl -s http://localhost:8000/
curl -s http://localhost:8000/help.sh
```

## 배포 (systemd)

```bash
# 서비스 파일 복사
sudo cp srv.service /etc/systemd/system/sh-server.service

# 환경 변수 설정 (ADMIN_TOKEN 수정 필수)
sudo systemctl edit sh-server.service
# [Service]
# Environment=ADMIN_TOKEN=your-real-secret

# 시작
sudo systemctl daemon-reload
sudo systemctl enable sh-server
sudo systemctl start sh-server

# 로그 확인
journalctl -u sh-server -f
```

## Docker 배포

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o sh-server ./cmd/srv

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/sh-server .
ENV PORT=8000
ENV DB_PATH=/data/sh.db
EXPOSE 8000
CMD ["./sh-server"]
```

```bash
docker build -t sh-server .
docker run -d -p 8000:8000 \
  -e ADMIN_TOKEN=secret \
  -e HOSTNAME=sh.huny.dev \
  -v sh-data:/data \
  sh-server
```

## 프로젝트 구조

```
sh-server/
├── cmd/srv/
│   └── main.go              # 엔트리포인트
├── srv/
│   ├── server.go            # HTTP 핸들러, 콘텐츠 협상
│   ├── api.go               # CRUD API
│   ├── static/
│   │   ├── style.css        # UI 스타일
│   │   └── app.js           # SPA 로직
│   └── templates/
│       └── index.html       # HTML 템플릿
├── db/
│   ├── db.go                # DB 초기화
│   ├── migrations/
│   │   └── 001-base.sql     # 스키마
│   ├── queries/             # SQL 쿼리
│   └── dbgen/               # sqlc 생성 코드
├── srv.service              # systemd 서비스
├── go.mod
└── README.md
```

## 고도화 아이디어

1. **Rate Limiting**: 잠금 해제 brute force 방지
2. **IP 바인딩**: 토큰을 IP에 바인딩
3. **스크립트 버전 관리**: 변경 이력 조회/롤백
4. **웹훅/알림**: 스크립트 실행 시 알림
5. **그룹/태그 권한**: 특정 태그 스크립트만 접근 가능
6. **스크립트 검증**: shellcheck 통합
7. **실행 통계**: 어떤 스크립트가 많이 사용되는지
8. **CLI 도구**: sh-cli로 로컬에서 스크립트 관리

## 라이선스

MIT
