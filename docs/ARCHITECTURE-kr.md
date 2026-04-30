# Rail Architecture

Rail은 Python-first 하네스 control plane이다. downstream application은 이 저장소에 들어오지 않으며, 별도 target repository를 artifact handle 중심으로 실행한다.

## 흐름

1. Rail skill이 자연어 요청을 request draft로 만든다.
2. `rail.specify(draft)`가 schema-valid request를 만든다.
3. `rail.start_task(draft)`가 fresh artifact handle을 할당한다.
4. `rail.supervise(handle)`이 supervisor graph와 Actor Runtime을 실행한다.
5. validation evidence와 evaluator gate를 통과한 결과만 terminal pass가 된다.
6. `rail.result(handle)`은 artifact만 읽어 결과를 projection한다.

## 정책

Rail/operator default policy가 먼저 적용되고 target policy는 좁히기만 할 수 있다. 알 수 없는 policy key, direct target mutation, target-local credential은 거부된다.

## 격리

Actor는 외부 sandbox에서 작업하고 target mutation은 Rail-validated patch bundle apply를 통해서만 발생한다.

## Artifact Identity

Artifact handle이 run identity다. Request file path는 run identity가 아니며, 사용자는 task id를 직접 선택하지 않는다.
