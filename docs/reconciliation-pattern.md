# Kubernetes 리소스 조정 패턴 (Reconciliation Pattern)

이 문서는 Kubernetes Operator에서 리소스를 효율적으로 관리하기 위한 공통 조정 패턴을 설명합니다.

## 패턴 개요

모든 Kubernetes 리소스에 대해 일관된 방식으로 다음 패턴을 적용합니다:

1. **리소스 존재 확인** → `client.Get()`
2. **수정 필요성 판단** → `DeepEqual()` 또는 필드 비교
3. **안전한 업데이트** → `ResourceVersion` 보존하여 `Update()`

## 핵심 함수들

### 1. `EnsureResource[T]` - 기본 조정 함수

```go
func EnsureResource[T client.Object](
    ctx context.Context,
    c client.Client,
    desired T,
    creator func(context.Context, client.Client, T) error,
    comparer func(T, T) bool,
    updater func(context.Context, client.Client, T, T) error,
) ReconcileResult
```

### 2. 편의 함수들

- `EnsureResourceWithDefaults[T]` - 기본 creator, comparer, updater 사용
- `EnsureResourceWithCustomComparer[T]` - 커스텀 비교 로직 사용
- `EnsureResourceWithCustomLogic[T]` - 모든 로직 커스터마이징

## 사용 예제

### 1. 기본 사용법 (DeepEqual 비교)

```go
func (r *MyReconciler) ReconcileConfigMap(ctx context.Context, obj *MyObject) error {
    desiredConfigMap := r.buildConfigMap(obj)
    
    result := util.EnsureResourceWithDefaults(
        ctx,
        r.Client,
        desiredConfigMap,
    )
    
    if result.Error != nil {
        return fmt.Errorf("failed to reconcile ConfigMap: %w", result.Error)
    }
    
    if result.Created {
        log.Info("ConfigMap created", "name", desiredConfigMap.Name)
    } else if result.Updated {
        log.Info("ConfigMap updated", "name", desiredConfigMap.Name)
    }
    
    return nil
}
```

### 2. 커스텀 비교 로직 사용

```go
func (r *MyReconciler) ReconcileService(ctx context.Context, obj *MyObject) error {
    desiredService := r.buildService(obj)
    
    // 특정 필드만 비교하는 커스텀 비교 함수
    customComparer := func(existing, desired *corev1.Service) bool {
        return existing.Spec.Type == desired.Spec.Type &&
               reflect.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) &&
               reflect.DeepEqual(existing.Spec.Selector, desired.Spec.Selector)
    }
    
    result := util.EnsureResourceWithCustomComparer(
        ctx,
        r.Client,
        desiredService,
        customComparer,
    )
    
    if result.Error != nil {
        return fmt.Errorf("failed to reconcile Service: %w", result.Error)
    }
    
    return nil
}
```

### 3. 완전 커스터마이징

```go
func (r *MyReconciler) ReconcileStatefulSet(ctx context.Context, obj *MyObject) error {
    desiredStatefulSet := r.buildStatefulSet(obj)
    
    result := util.EnsureResourceWithCustomLogic(
        ctx,
        r.Client,
        desiredStatefulSet,
        r.createStatefulSet,    // 커스텀 생성 함수
        r.isStatefulSetEqual,   // 커스텀 비교 함수
        r.updateStatefulSet,    // 커스텀 업데이트 함수
    )
    
    if result.Error != nil {
        return fmt.Errorf("failed to reconcile StatefulSet: %w", result.Error)
    }
    
    return nil
}

// 커스텀 생성 함수
func (r *MyReconciler) createStatefulSet(ctx context.Context, c client.Client, sts *appsv1.StatefulSet) error {
    // Owner reference 설정
    if err := controllerutil.SetControllerReference(r.owner, sts, r.Scheme); err != nil {
        return fmt.Errorf("failed to set controller reference: %w", err)
    }
    return c.Create(ctx, sts)
}

// 커스텀 비교 함수
func (r *MyReconciler) isStatefulSetEqual(existing, desired *appsv1.StatefulSet) bool {
    // 복제본 수 비교
    if existing.Spec.Replicas == nil && desired.Spec.Replicas != nil {
        return false
    }
    if existing.Spec.Replicas != nil && desired.Spec.Replicas != nil && 
       *existing.Spec.Replicas != *desired.Spec.Replicas {
        return false
    }
    
    // 템플릿 비교
    return reflect.DeepEqual(existing.Spec.Template, desired.Spec.Template)
}

// 커스텀 업데이트 함수
func (r *MyReconciler) updateStatefulSet(ctx context.Context, c client.Client, existing, desired *appsv1.StatefulSet) error {
    // ResourceVersion 보존
    desired.SetResourceVersion(existing.GetResourceVersion())
    
    // Status 보존
    desired.Status = existing.Status
    
    return c.Update(ctx, desired)
}
```

## 패턴의 장점

### 1. **일관성**
- 모든 리소스에 대해 동일한 패턴 적용
- 코드 가독성과 유지보수성 향상

### 2. **안전성**
- ResourceVersion 보존으로 동시성 충돌 방지
- 명시적인 비교 로직으로 예측 가능한 동작

### 3. **효율성**
- 필요한 경우에만 업데이트 수행
- 불필요한 API 호출 최소화

### 4. **확장성**
- 새로운 리소스 타입에 쉽게 적용 가능
- 커스텀 로직으로 세밀한 제어 가능

## 구현된 매니저들

### 1. StatefulSet Manager
- `ReconcileStatefulSet()` - 새로운 패턴 적용
- `isStatefulSetEqual()` - StatefulSet 특화 비교 로직
- `updateStatefulSet()` - 복제본 동기화 및 안전한 업데이트

### 2. Service Manager
- `ReconcileServices()` - 여러 서비스 일괄 조정
- `isServiceEqual()` - 서비스 특화 비교 로직
- Headless, ClusterIP, CMS 서비스 지원

## 모범 사례

### 1. 비교 함수 작성
```go
// 좋은 예: 명시적인 필드 비교
func (r *Reconciler) isPodEqual(existing, desired *corev1.Pod) bool {
    // 이미지 비교
    if existing.Spec.Containers[0].Image != desired.Spec.Containers[0].Image {
        return false
    }
    
    // 리소스 비교
    if !reflect.DeepEqual(existing.Spec.Containers[0].Resources, 
                         desired.Spec.Containers[0].Resources) {
        return false
    }
    
    return true
}

// 피해야 할 예: 전체 객체 비교
func (r *Reconciler) isPodEqual(existing, desired *corev1.Pod) bool {
    return reflect.DeepEqual(existing, desired) // 너무 광범위함
}
```

### 2. 업데이트 함수 작성
```go
// 좋은 예: 필요한 필드만 보존
func (r *Reconciler) updatePod(ctx context.Context, c client.Client, existing, desired *corev1.Pod) error {
    desired.SetResourceVersion(existing.GetResourceVersion())
    desired.Status = existing.Status
    desired.Spec.NodeName = existing.Spec.NodeName // 노드 할당 보존
    
    return c.Update(ctx, desired)
}
```

### 3. 에러 처리
```go
result := util.EnsureResourceWithDefaults(ctx, r.Client, desiredObj)
if result.Error != nil {
    return fmt.Errorf("failed to reconcile %s: %w", 
                     reflect.TypeOf(desiredObj).Elem().Name(), result.Error)
}

// 결과에 따른 로깅
if result.Created {
    log.Info("Resource created", "kind", "Pod", "name", desiredObj.GetName())
} else if result.Updated {
    log.Info("Resource updated", "kind", "Pod", "name", desiredObj.GetName())
}
```

## 결론

이 패턴을 사용하면 Kubernetes Operator의 리소스 관리가 더욱 안전하고 효율적이며 일관성 있게 됩니다. 모든 리소스 타입에 대해 동일한 패턴을 적용하여 코드의 복잡성을 줄이고 유지보수성을 향상시킬 수 있습니다. 