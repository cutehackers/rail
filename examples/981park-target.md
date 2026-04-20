# Example Target Bootstrap

예시용 대상 저장소:

- `/absolute/path/to/target-app`

일반 사용자 기준의 제품 경로:

```bash
brew install rail
cd /absolute/path/to/target-app
rail init
```

그 다음의 일반적인 진입점은 Rail skill입니다.

예시 프롬프트:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-app
Goal: 설명 가능한 버그를 재현 가능한 수정 단위로 정의해줘.
Constraint: 영향 범위를 좁게 유지해.
Definition of done: 관련 검증 범위가 명확해지고 focused validation 경로가 남아야 해.
```

고급 사용자가 request materialization만 직접 확인하고 싶다면 그때만 draft와 `rail compose-request`를 사용할 수 있습니다. 그것은 보조 경로이지, 일반 사용자의 기본 운영 모델은 아닙니다.
