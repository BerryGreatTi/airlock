# macOS E2E Testing: GUI-Primary Redesign (Tasks 6-15)

PR #1이 main에 머지되었다. Go/Python 테스트는 Linux에서 통과 확인 완료. macOS에서 GUI 빌드 검증과 수동 E2E 테스트를 수행하라.

## 1단계: 빌드 및 유닛 테스트

```bash
git pull origin main
make gui-build && make gui-test
```

빌드 실패 시 에러를 분석하고 수정하라. Swift 6 concurrency 경고가 있으면 보고하라.

## 2단계: 수동 E2E 테스트

앱을 실행하고 아래 체크리스트를 순서대로 검증하라. 각 항목마다 결과를 PASS/FAIL로 기록하라.

### 사전 조건
- Docker Desktop 실행 중
- `make docker-build` 완료 (`airlock-claude:latest`, `airlock-proxy:latest` 이미지 존재)
- 테스트용 디렉토리 생성: `mkdir -p /tmp/airlock-e2e-test && cd /tmp/airlock-e2e-test && airlock init`
- 테스트용 .env 파일: `/tmp/airlock-e2e-test/.env` 에 `STRIPE_KEY=sk_test_fake123` 한 줄 작성

### 체크리스트

| # | 테스트 | 검증 방법 | 결과 |
|---|---|---|---|
| 1 | Welcome 화면 | workspace가 없는 상태에서 앱 실행. Docker 상태 체크(녹색 체크 or 빨간 X) + "Create Your First Workspace" 버튼 표시 확인 | |
| 2 | Workspace 생성 pre-check | "Create Your First Workspace" 클릭 -> 디렉토리로 `/tmp/airlock-e2e-test` 선택, .env 경로 입력. pre-check 4항목 표시 확인: directory exists, .airlock/ initialized, Docker running, plaintext secrets detected | |
| 3 | Workspace 생성 | "Add Workspace" 클릭 -> 사이드바에 워크스페이스 나타남 확인 | |
| 4 | Activate (사이드바) | 워크스페이스 우클릭 -> "Activate" -> 녹색 dot 표시 + `docker ps`에서 `airlock-claude-*`, `airlock-proxy-*` 컨테이너 running 확인 | |
| 5 | Terminal 탭 | 활성화 후 Terminal 탭에서 bash 셸 진입 확인. `whoami` 또는 `ls` 명령 실행 가능 확인 | |
| 6 | 터미널 분할 - Cmd+T | Cmd+T -> 새 터미널 pane 추가, 카운터 "2 terminals" 표시 확인 | |
| 7 | 터미널 분할 - Cmd+D | Cmd+D -> 수직 분할 + 새 pane 추가 확인 | |
| 8 | 터미널 분할 - Cmd+Shift+D | Cmd+Shift+D -> 수평 분할 + 새 pane 추가 확인 | |
| 9 | 탭 전환 단축키 | Cmd+1(Terminal), Cmd+2(Secrets), Cmd+3(Containers), Cmd+4(Diff), Cmd+5(Settings) 전환 확인 | |
| 10 | Secrets 탭 | Cmd+2 -> .env 파일 로드 확인. STRIPE_KEY가 "Plaintext" 상태(주황색 dot)로 표시 확인 | |
| 11 | Secrets - Encrypt All | "Encrypt All" 버튼 클릭 -> STRIPE_KEY 상태가 "Encrypted" (녹색 dot)로 변경 확인. "Restart workspace to apply changes" 배너 표시 확인 | |
| 12 | Secrets - Add Entry | "Add Entry" 클릭 -> Key: `TEST_VAR`, Value: `hello` 입력 -> 테이블에 추가 확인 | |
| 13 | Containers 탭 | Cmd+3 -> Agent/Proxy/Network 카드 3개 표시 확인. 각 카드에 컨테이너 이름 표시 확인 | |
| 14 | Proxy activity log | Containers 탭 하단 "Proxy Activity Log" 영역 확인. 컨테이너 내에서 `curl https://httpbin.org/get` 실행 후 로그에 항목 나타남 확인 | |
| 15 | 메뉴바 Activate/Deactivate | Workspace 메뉴 -> "Deactivate" (Cmd+.) -> 컨테이너 정지 확인. 다시 "Activate" (Cmd+R) -> 컨테이너 재시작 확인 | |
| 16 | Deactivate 확인 다이얼로그 | 활성 워크스페이스에서 Deactivate 시 "This will stop all containers" 다이얼로그 표시 확인 | |
| 17 | Crash recovery | 워크스페이스 활성 상태에서 앱 강제 종료 (Activity Monitor에서 kill). 앱 재실행 -> 실행 중인 컨테이너 자동 감지 + 녹색 dot 복원 확인 | |
| 18 | Orphan cleanup | 앱 종료 후 워크스페이스를 수동 삭제(앱 설정 파일에서). 컨테이너는 running 유지. 앱 재실행 -> "Orphaned Containers Found" 다이얼로그 -> "Clean Up" 클릭 -> 컨테이너 정지 확인 | |
| 19 | 멀티 워크스페이스 | 두 번째 테스트 디렉토리 생성, 두 번째 워크스페이스 추가 후 양쪽 모두 Activate. 두 워크스페이스 모두 녹색 dot, `docker ps`에 4개 컨테이너(claude x2, proxy x2) 확인 | |
| 20 | Stop and Remove | 워크스페이스 우클릭 -> "Stop and Remove" -> 확인 다이얼로그 -> 컨테이너 정지 + 사이드바에서 제거 확인 | |

## 3단계: 정리

```bash
# 테스트 컨테이너 정리
airlock stop
rm -rf /tmp/airlock-e2e-test
```

## 보고 형식

테스트 완료 후 아래 형식으로 결과를 보고하라:

```
## E2E Test Results
- Date: YYYY-MM-DD
- macOS version:
- Xcode version:
- Build: PASS/FAIL
- Unit tests: PASS/FAIL (N tests)
- E2E: N/20 PASS

### FAIL 항목 (있는 경우)
- #N: 설명 + 에러 메시지/스크린샷
```

FAIL 항목이 있으면 원인 분석 후 수정 커밋을 만들어라. 수정 후 해당 항목을 재테스트하라.
