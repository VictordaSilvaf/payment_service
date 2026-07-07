# Payment Service

API REST em Go para gerenciamento de pagamentos, construída com **Domain-Driven Design (DDD)** e **Arquitetura Hexagonal** (Ports & Adapters).

---

## Sumário

- [Visão geral](#visão-geral)
- [Stack tecnológica](#stack-tecnológica)
- [Arquitetura](#arquitetura)
- [Estrutura de pastas](#estrutura-de-pastas)
- [Fluxogramas](#fluxogramas)
- [Endpoints da API](#endpoints-da-api)
- [Idempotência](#idempotência)
- [Como rodar](#como-rodar)
- [Testes](#testes)
- [Migrations, factories e seeders](#migrations-factories-e-seeders)
- [Variáveis de ambiente](#variáveis-de-ambiente)
- [Banco de dados](#banco-de-dados)
- [Docker](#docker)

---

## Visão geral

O **Payment Service** expõe uma API HTTP para criar, consultar, listar, **capturar** e **estornar** pagamentos. Cada pagamento possui valor (`amount`), moeda (`currency`), número de parcelas (`installments`), método de captura (`capture_method`) e status (`pending`, `authorized`, `completed`, `failed`, `refunded`, `partially_refunded`).

Principais capacidades:

- **Persistência** no PostgreSQL (adapter `pgx`)
- **Idempotência** no `POST /payments` via Redis (header `Idempotency-Key`)
- **Eventos** publicados no RabbitMQ ao criar um pagamento (`payment.created`)
- **Outbox Pattern**: o evento é gravado na mesma transação do pagamento e publicado depois por um relay (sem dual-write)
- **Consumer** assíncrono que consome `payment.created`, **autoriza no PSP** e transiciona o pagamento, emitindo `payment.completed`, `payment.authorized` ou `payment.failed`
- **Captura automática ou manual** (`capture_method`): na automática o pagamento é liquidado logo após a autorização (`pending` → `completed`); na manual a autorização apenas reserva os fundos (`pending` → `authorized`) e a captura é disparada depois por `POST /payments/:id/capture` (`authorized` → `completed`)
- **Estorno** total ou parcial (`POST /payments/:id/refund`): `completed` → `partially_refunded` → `refunded`, emitindo `payment.refunded`
- **Parcelamento** (`installments`, 1–12) registrado no pagamento
- **PSP Mock**: gateway simulado que aprova/recusa a autorização de forma determinística (par aprova, ímpar recusa) e sempre aceita captura/estorno, com latência configurável — a implementação real (Stripe/Adyen) entraria no lugar sem alterar o núcleo
- **Webhook Service**: notifica endpoints externos (lojistas) via POST assinado (HMAC) quando um pagamento é concluído, recusado ou estornado, com **retry** (backoff exponencial + limite de tentativas) para entregas que falharam
- **Notification Service**: notifica o **usuário final** (e-mail/SMS/push — mock via log) quando o pagamento é concluído, recusado ou estornado, com dedup por id determinístico e reentrega via DLQ
- **Testes** com cobertura mínima de 80% nos pacotes de negócio

---

## Stack tecnológica

| Tecnologia | Uso |
|---|---|
| **Go 1.26** | Linguagem principal |
| **Gin** | Framework HTTP |
| **pgx/v5** | Driver PostgreSQL |
| **go-redis/v9** | Cliente Redis (idempotência) |
| **golang-migrate** | Migrations versionadas |
| **PostgreSQL 16** | Persistência de pagamentos |
| **Redis 7** | Cache de idempotência (lock + resposta) |
| **RabbitMQ 3.13** | Publicação e consumo de eventos (`payment.created`, `payment.completed`) |
| **amqp091-go** | Cliente AMQP |
| **HMAC-SHA256** | Assinatura das entregas de webhook |
| **Docker Compose** | Orquestração de containers |
| **Air** | Hot-reload em desenvolvimento |
| **miniredis** | Redis em memória para testes |

---

## Arquitetura

O projeto segue **Arquitetura Hexagonal** com separação clara de responsabilidades:

```mermaid
flowchart TB
    subgraph Driving["Adapters de Entrada (Driving)"]
        HTTP["HTTP Handler\n(Gin)"]
        MW["Middleware\n(Idempotency-Key)"]
    end

    subgraph Application["Camada de Aplicação"]
        UC["Use Cases\n(Create, Get, List)"]
        IDEM["Idempotency Service"]
        DTO["DTOs / Mappers"]
    end

    subgraph Domain["Domínio (Núcleo)"]
        Entity["Entidade Payment"]
        VO["Value Objects\n(Money, Status)"]
        Port["Portas\n(Repository, EventPublisher)"]
    end

    subgraph Driven["Adapters de Saída (Driven)"]
        PG["PostgreSQL Repository"]
        RD["Redis Idempotency Repository"]
        RMQ["RabbitMQ Publisher"]
        MEM["Memory Repository\n(testes)"]
    end

    HTTP --> MW
    MW --> IDEM
    IDEM --> UC
    UC --> DTO
    UC --> Port
    IDEM --> RD
    Port -.->|implementa| PG
    Port -.->|implementa| MEM
    Port -.->|implementa| RMQ
    UC --> Entity
    Entity --> VO
```

### Princípios

| Camada | Responsabilidade | Depende de |
|---|---|---|
| **Domain** | Regras de negócio, entidades, value objects, contratos (ports) | Nada externo |
| **Application** | Orquestração via use cases, idempotência, DTOs e mappers | Domain |
| **Infrastructure** | HTTP, banco de dados, Redis, RabbitMQ, config | Application + Domain |

> O domínio e a aplicação **nunca** importam Gin, PostgreSQL ou Redis diretamente. Apenas os entrypoints em `cmd/` fazem o wiring das dependências (composition root).

---

## Estrutura de pastas

```
payment_service/
├── cmd/
│   ├── api/main.go                          # Composition root da API
│   ├── consumer/main.go                     # Composition root do consumer RabbitMQ
│   ├── outbox/main.go                       # Composition root do relay do outbox
│   ├── webhook/main.go                      # Composition root do webhook service
│   ├── notification/main.go                 # Composition root do notification service
│   ├── migrate/main.go                      # CLI de migrations
│   └── seed/main.go                         # CLI de seeders
├── db/migrations/                           # SQL versionado (golang-migrate)
├── internal/
│   ├── domain/payment/                      # Núcleo do domínio
│   │   ├── payment.go                       # Entidade + transições (autorizar, capturar, estornar)
│   │   ├── money.go                         # Value object
│   │   ├── status.go                        # Value object (estados do pagamento)
│   │   ├── capture_method.go               # Value object (automatic/manual)
│   │   ├── page.go                          # Resultado paginado
│   │   ├── publisher.go                     # Porta EventPublisher
│   │   ├── errors.go
│   │   └── repository.go                    # Porta Repository
│   ├── domain/outbox/                       # Evento e porta do Outbox
│   │   ├── event.go                         # Modelo Event
│   │   └── repository.go                    # Porta Repository
│   ├── domain/psp/                          # Porta Gateway do PSP + tipos de resultado
│   ├── domain/webhook/                      # Assinatura, entrega, assinatura HMAC e portas
│   │   ├── subscription.go                  # Entidade Subscription
│   │   ├── delivery.go                      # Entidade Delivery
│   │   ├── signature.go                     # Assinatura HMAC-SHA256
│   │   ├── sender.go                        # Porta Sender
│   │   └── repository.go                    # Portas Subscription/Delivery
│   ├── domain/notification/                 # Notificação ao usuário + portas Notifier/Repository
│   ├── application/
│   │   ├── dto/                             # Objetos de transferência
│   │   ├── idempotency/                     # Serviço e contratos de idempotência
│   │   ├── outbox/                          # Relay (dispatcher) + porta Publisher
│   │   ├── webhook/                         # DispatchWebhook + RetryDeliveries + gestão de assinaturas
│   │   ├── notification/                    # NotifyPayment (monta e envia a notificação)
│   │   ├── payment/                         # Mapper e validator
│   │   └── usecase/                         # Casos de uso (criar, buscar, listar, processar, capturar, estornar)
│   ├── database/
│   │   ├── migrate/                         # Runner de migrations
│   │   ├── factory/                         # Factories de dados fake
│   │   └── seeder/                          # Seeders
│   ├── testutil/                            # Mocks compartilhados nos testes
│   └── infrastructure/
│       ├── config/
│       ├── cache/redis/                     # Cliente Redis + idempotência
│       ├── http/                            # Router, handlers, middleware
│       │   └── webhookclient/               # Sender HTTP (entrega de webhooks)
│       ├── messaging/rabbitmq/              # Adapter RabbitMQ (publisher, consumer, subscriber, DLQ)
│       ├── psp/                             # Adapter PSP mock (autorização)
│       ├── notification/                    # Adapter Notifier mock (log)
│       └── persistence/
│           ├── postgres/                    # Adapter PostgreSQL (payments, outbox, webhooks, notifications, TxManager)
│           └── memory/                      # Adapter in-memory (testes)
├── docker-compose.yml                       # Orquestração (produção)
├── docker-compose.override.yml              # Hot-reload (dev, auto-carregado)
├── Dockerfile                               # Build de produção (API + consumer + outbox + webhook + notification)
├── Dockerfile.dev                           # Build de desenvolvimento (Air)
├── .air.toml                                # Hot-reload da API
├── .air.consumer.toml                       # Hot-reload do consumer
├── .air.outbox.toml                         # Hot-reload do relay do outbox
├── .air.webhook.toml                        # Hot-reload do webhook service
└── Makefile                                 # Atalhos (migrate, seed, test, run)
```

---

## Fluxogramas

### Fluxo de uma requisição — Criar pagamento (com idempotência)

```mermaid
sequenceDiagram
    participant Client as Cliente HTTP
    participant MW as Idempotency Middleware
    participant Handler as PaymentHandler
    participant IDEM as Idempotency Service
    participant Redis as Redis
    participant UC as CreatePayment
    participant Domain as payment.New()
    participant TX as TxManager
    participant Repo as PostgresRepository
    participant Outbox as OutboxRepository
    participant DB as PostgreSQL

    Client->>MW: POST /api/v1/payments\nIdempotency-Key: uuid
    MW->>MW: Valida header
    MW->>Handler: idempotency_key no contexto
    Handler->>IDEM: Execute(key, hash, fn)
    IDEM->>Redis: Find(key)
    alt Cache hit
        Redis-->>IDEM: resposta cacheada
        IDEM-->>Handler: CachedResponse
        Handler-->>Client: 201 (mesma resposta)
    else Cache miss
        IDEM->>Redis: Lock(key)
        IDEM->>UC: Execute(ctx, request)
        UC->>Domain: New(amount, currency)
        Domain-->>UC: *Payment
        UC->>TX: WithinTx(ctx, fn)
        TX->>Repo: Save(ctx, payment)
        Repo->>DB: INSERT INTO payments
        TX->>Outbox: Add(ctx, event)
        Outbox->>DB: INSERT INTO outbox_events
        Note over TX,DB: mesma transação (atômico)
        UC-->>IDEM: PaymentResponse
        IDEM->>Redis: Save(key, response)
        IDEM->>Redis: Unlock(key)
        IDEM-->>Handler: CachedResponse
        Handler-->>Client: 201 Created
    end
```

### Fluxo de uma requisição — Listar pagamentos

```mermaid
sequenceDiagram
    participant Client as Cliente HTTP
    participant Handler as PaymentHandler
    participant UC as ListPayment
    participant Repo as PostgresRepository
    participant DB as PostgreSQL

    Client->>Handler: GET /api/v1/payments?page=1&limit=10
    Handler->>UC: Execute(ctx, page, limit, sort, order, status)
    UC->>Repo: FindAll(ctx, params...)
    Repo->>DB: COUNT(*) + SELECT ... LIMIT/OFFSET
    DB-->>Repo: total + rows
    Repo-->>UC: PageResult{Items, Total}
    UC->>UC: Monta PaginatedResponse
    UC-->>Handler: *PaginatedResponse
    Handler-->>Client: 200 OK
```

### Fluxo assíncrono — Publicar evento (Outbox Relay)

```mermaid
sequenceDiagram
    participant Relay as Outbox Relay
    participant Outbox as OutboxRepository
    participant DB as PostgreSQL
    participant RMQ as RabbitMQ

    loop a cada OUTBOX_POLL_INTERVAL
        Relay->>Outbox: FetchUnpublished(batch)
        Outbox->>DB: SELECT ... WHERE published_at IS NULL
        DB-->>Relay: eventos pendentes
        loop cada evento
            Relay->>RMQ: Publish(routingKey, payload)
            Relay->>Outbox: MarkPublished(id)
            Outbox->>DB: UPDATE outbox_events SET published_at
        end
    end
```

> Entrega **at-least-once**: se o relay cair após publicar e antes de marcar, o evento é republicado no próximo ciclo. Por isso o consumer deve ser idempotente.

### Fluxo assíncrono — Processar pagamento (consumer)

```mermaid
sequenceDiagram
    participant RMQ as RabbitMQ (fila payment)
    participant Consumer as PaymentConsumer
    participant UC as ProcessPayment
    participant Domain as payment.Complete()
    participant Repo as PostgresRepository
    participant PSP as PSP (mock)
    participant DB as PostgreSQL

    RMQ->>Consumer: entrega payment.created
    Consumer->>UC: Execute(ctx, event)
    UC->>Repo: FindByID(ctx, id)
    Repo->>DB: SELECT ... WHERE id
    UC->>PSP: Authorize(ctx, payment)
    alt Aprovado
        PSP-->>UC: approved
        UC->>Domain: Complete()  → status = completed
        Note over UC,DB: Mesma transação (Outbox)
        UC->>Repo: Update(ctx, payment)
        UC->>DB: INSERT outbox_events (payment.completed)
        Consumer->>RMQ: Ack
    else Recusado
        PSP-->>UC: declined
        UC->>Domain: Fail()  → status = failed
        Note over UC,DB: Mesma transação (Outbox)
        UC->>Repo: Update(ctx, payment)
        UC->>DB: INSERT outbox_events (payment.failed)
        Consumer->>RMQ: Ack
    else Erro do PSP (timeout/indisponível)
        PSP-->>UC: error
        Note over UC,Consumer: retenta até RABBITMQ_MAX_RETRIES (pagamento continua pending)
        alt Ainda dentro do limite
            Consumer->>RMQ: aguarda RETRY_DELAY e tenta de novo
        else Esgotou as tentativas
            Consumer->>RMQ: Nack(requeue=false) → DLQ (payment.created.dlq)
        end
    end
```

> Se o consumer **não estiver rodando**, os eventos ficam acumulados na fila e os pagamentos permanecem em `pending` até um consumer conectar e processá-los.

> A **recusa** do PSP é uma decisão de negócio (não é erro): o pagamento vira `failed` e o evento `payment.failed` é emitido. Já um **erro** do PSP (timeout/indisponível) é transitório: o consumer retenta algumas vezes e, se persistir, manda a mensagem para a **DLQ** em vez de reenfileirar para sempre (ver seção *Dead Letter Queue*).

### Ciclo de vida do pagamento (estados)

Captura **automática** liquida logo após a autorização; captura **manual** passa por `authorized` e exige uma chamada explícita de captura. Depois de capturado, o pagamento pode ser estornado (total ou parcialmente).

```mermaid
stateDiagram-v2
    [*] --> pending: criar
    pending --> completed: autoriza (captura automática)
    pending --> authorized: autoriza (captura manual)
    pending --> failed: recusado pelo PSP
    authorized --> completed: capturar
    completed --> partially_refunded: estorno parcial
    completed --> refunded: estorno total
    partially_refunded --> partially_refunded: estorno parcial
    partially_refunded --> refunded: estorno do restante
    failed --> [*]
    refunded --> [*]
```

### Fluxo assíncrono — Entregar webhook (Webhook Service)

```mermaid
sequenceDiagram
    participant RMQ as RabbitMQ (fila webhook.payment)
    participant Sub as Subscriber
    participant UC as DispatchWebhook
    participant SubsRepo as SubscriptionRepository
    participant Sender as HTTP Sender
    participant Merchant as Endpoint do lojista
    participant DelRepo as DeliveryRepository

    RMQ->>Sub: entrega payment.completed
    Sub->>UC: Execute(ctx, "payment.completed", payload)
    UC->>SubsRepo: FindActiveByEventType(ctx, type)
    loop cada assinatura ativa
        UC->>Sender: POST payload + X-Webhook-Signature (HMAC)
        Sender->>Merchant: HTTP POST
        Merchant-->>Sender: 2xx / erro
        alt 2xx
            UC->>DelRepo: Save(delivery = delivered)
        else erro / não-2xx
            UC->>DelRepo: Save(delivery = failed, next_attempt_at = agora + backoff)
        end
    end
    alt Erro de infraestrutura (buscar assinatura / salvar entrega)
        Sub->>RMQ: Nack (reenfileira)
    else
        Sub->>RMQ: Ack
    end
```

> Falha HTTP de um endpoint é registrada como entrega `failed` **com o próximo horário de tentativa agendado** e **não** derruba o lote. Só erros de infraestrutura devolvem a mensagem à fila. O id determinístico enviado em `X-Webhook-Id` permite ao lojista deduplicar reentregas.

### Fluxo assíncrono — Retry de entregas (Webhook Service)

Um poller roda **dentro do próprio webhook service**, em paralelo ao subscriber (padrão semelhante ao relay do outbox): a cada `WEBHOOK_RETRY_POLL_INTERVAL` ele varre as entregas `failed` cujo `next_attempt_at` já venceu e as reenvia, com **backoff exponencial** e limite de tentativas.

```mermaid
sequenceDiagram
    participant Retry as RetryDeliveries (poller)
    participant DelRepo as DeliveryRepository
    participant Sender as HTTP Sender
    participant Merchant as Endpoint do lojista

    loop a cada WEBHOOK_RETRY_POLL_INTERVAL
        Retry->>DelRepo: FetchRetriable(limit, now)  // status=failed AND next_attempt_at<=now (join assinatura ativa)
        loop cada entrega elegível
            Retry->>Sender: POST payload + assinatura (recalculada com o segredo atual)
            Sender->>Merchant: HTTP POST
            Merchant-->>Sender: 2xx / erro
            alt 2xx
                Retry->>DelRepo: Save(delivered)
            else falha e ainda dentro do limite
                Retry->>DelRepo: Save(failed, next_attempt_at = agora + base·2^(n-1))
            else falha e atingiu MAX_ATTEMPTS
                Retry->>DelRepo: Save(exhausted)  // terminal, não retenta mais
            end
        end
    end
```

Pontos importantes do desenho:

- **Auto-contido**: a entrega guarda `event_type` e `payload`, então o retry independe da mensagem original do RabbitMQ. URL e segredo vêm de um *join* com a assinatura (usa a config **atual** do lojista).
- **Idempotente**: o `X-Webhook-Id` continua determinístico entre tentativas — o lojista deduplica reenvios.
- **Backoff exponencial**: `base`, `base·2`, `base·4`, ... controlado por `WEBHOOK_RETRY_BASE_DELAY`, evitando martelar um endpoint fora do ar.
- **Estado terminal `exhausted`**: ao estourar `WEBHOOK_RETRY_MAX_ATTEMPTS`, a entrega sai da fila de retry — o equivalente à DLQ, porém no nível do banco.

### Fluxo assíncrono — Dead Letter Queue (mensagens do broker)

Enquanto o *Retry de webhook* atua no **banco** (entregas HTTP), a **DLQ** protege o consumo de **mensagens do RabbitMQ**. Antes, uma falha de processamento fazia `Nack(requeue=true)` — a mensagem voltava ao início da fila e podia rodar em loop infinito (*poison message*); e mensagens malformadas eram simplesmente descartadas (perdidas).

Agora, cada fila principal é declarada com uma **Dead Letter Exchange (DLX)** e uma **Dead Letter Queue** (`<fila>.dlq`). O consumer/subscriber retenta o processamento algumas vezes e, se não conseguir, **rejeita sem reenfileirar** (`Nack(requeue=false)`) — o RabbitMQ então roteia a mensagem para a DLQ.

```mermaid
flowchart LR
    Ex["Exchange\npayment.events"] -->|payment.created| Q["Fila principal\n(payment.created)"]
    Q --> C["Consumer"]
    C -->|sucesso| Ack["Ack ✅"]
    C -->|"falha transitória"| R["retenta N×\n(RETRY_DELAY entre tentativas)"]
    R -->|"recuperou"| Ack
    R -->|"esgotou / malformada"| Nack["Nack(requeue=false)"]
    Nack -->|dead-letter| DLX["DLX\npayment.events.dlx"]
    DLX -->|routing key = fila| DLQ["DLQ\npayment.created.dlq\n(parking lot)"]
```

Como funciona:

- **DLX + DLQ por fila**: `declareDLQ` cria a exchange `<exchange>.dlx` (tipo *direct*) e a fila `<fila>.dlq`, ligadas pela routing key = nome da fila. A fila principal recebe os argumentos `x-dead-letter-exchange` e `x-dead-letter-routing-key`.
- **Retentativas antes da DLQ**: `RABBITMQ_MAX_RETRIES` tentativas com `RABBITMQ_RETRY_DELAY` entre elas. Só depois de esgotar é que a mensagem vai para a DLQ.
- **Malformada = falha permanente**: JSON inválido vai **direto** para a DLQ (não adianta retentar), em vez de ser descartado como antes.
- **Parking lot**: a DLQ é um "estacionamento" sem consumidor automático — dá para inspecionar, corrigir e reprocessar as mensagens manualmente.
- Vale para as duas filas: `payment.created` (consumer) e `webhook.payment` (webhook service) têm, cada uma, sua DLQ.

> ⚠️ Os argumentos de uma fila são **imutáveis**. Como as filas passaram a ter DLX, uma fila antiga declarada sem esses argumentos causa `PRECONDITION_FAILED` na subida. Em desenvolvimento, recrie o broker limpo: `docker compose down -v` (ou remova as filas pelo painel do RabbitMQ).

Inspecionar/drenar a DLQ pelo painel de gerenciamento (`http://localhost:15672`) ou via CLI:

```bash
# Quantas mensagens estão paradas em cada DLQ
docker exec payment_rabbitmq rabbitmqctl list_queues name messages | grep '\.dlq'
```

### Fluxo assíncrono — Notificar usuário (Notification Service)

O Notification Service consome os mesmos eventos do webhook (`payment.completed`/`payment.failed`), mas o destinatário é o **usuário final** (não o lojista). Ele monta uma mensagem legível e a "envia" por um canal (mock via log). Reaproveita o `Subscriber` — então já herda **retry + DLQ** de graça.

```mermaid
sequenceDiagram
    participant RMQ as RabbitMQ (fila notification.payment)
    participant Sub as Subscriber
    participant UC as NotifyPayment
    participant Notifier as Notifier (mock/log)
    participant Repo as NotificationRepository

    RMQ->>Sub: entrega payment.completed / payment.failed
    Sub->>UC: Execute(ctx, routingKey, payload)
    UC->>UC: monta mensagem + id determinístico (dedup)
    UC->>Notifier: Send(notification)
    alt Enviado
        Notifier-->>UC: ok
        UC->>Repo: Save(status = sent)
        Sub->>RMQ: Ack
    else Falha de envio
        Notifier-->>UC: erro
        UC->>Repo: Save(status = failed)
        UC-->>Sub: erro
        Sub->>RMQ: retenta N× e, se persistir, → DLQ
    end
```

Pontos importantes:

- **Dedup**: o id da notificação é determinístico por `(pagamento, evento, canal)` — reprocessar o mesmo evento faz upsert, sem notificar o usuário duas vezes.
- **Canal plugável**: a porta `Notifier` isola o meio (email/SMS/push). Hoje há um `LogNotifier` (mock); trocar por SendGrid/Twilio é só uma nova implementação.
- **Confiabilidade**: falha de envio é gravada como `failed` e o erro é propagado ao `Subscriber`, que retenta (`RABBITMQ_MAX_RETRIES`) e, se persistir, manda à DLQ `notification.payment.dlq`.
- **Destinatário**: como o pagamento não guarda dados do cliente, o mock deriva um e-mail de exemplo (`customer+<id>@example.com`). Num sistema real viria do cadastro do cliente.

### Infraestrutura Docker

```mermaid
flowchart LR
    subgraph Host["Máquina local"]
        Client["Cliente\n(curl / Postman)"]
    end

    subgraph Docker["Docker Compose"]
        API["payment_api\n(Go + Air)"]
        Outbox["payment_outbox\n(Go + Air)"]
        Consumer["payment_consumer\n(Go + Air)"]
        Webhook["payment_webhook\n(Go + Air)"]
        PG["payment_postgres\n(PostgreSQL 16)"]
        RD["payment_redis\n(Redis 7)"]
        RMQ["payment_rabbitmq\n(RabbitMQ 3.13)"]
    end

    Merchant["Endpoint do lojista"]

    Client -->|:8080| API
    API -->|:5432| PG
    API -->|:6379| RD
    Outbox -->|lê pendentes :5432| PG
    Outbox -->|publica :5672| RMQ
    RMQ -->|payment.created| Consumer
    Consumer -->|:5432| PG
    RMQ -->|payment.completed| Webhook
    Webhook -->|:5432| PG
    Webhook -->|POST assinado| Merchant

    PG --- VolPG[("postgres_data\n(volume)")]
    RD --- VolRD[("redis_data\n(volume)")]
    RMQ --- VolRMQ[("rabbitmq_data\n(volume)")]
```

---

## Endpoints da API

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/ping` | Health check |
| `POST` | `/api/v1/payments` | Criar pagamento (**requer** `Idempotency-Key`) |
| `GET` | `/api/v1/payments/:id` | Buscar pagamento por ID |
| `GET` | `/api/v1/payments` | Listar pagamentos (paginado) |
| `POST` | `/api/v1/payments/:id/capture` | Capturar um pagamento autorizado (captura manual) |
| `POST` | `/api/v1/payments/:id/refund` | Estornar um pagamento (total ou parcial) |
| `POST` | `/api/v1/webhooks` | Registrar assinatura de webhook |
| `GET` | `/api/v1/webhooks` | Listar assinaturas de webhook |

### Exemplos

**Health check**

```bash
curl http://localhost:8080/ping
# {"message":"pong"}
```

**Criar pagamento**

`installments` (padrão `1`) e `capture_method` (`automatic` padrão, ou `manual`) são opcionais.

```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"amount": 1000, "currency": "BRL", "installments": 3, "capture_method": "manual"}'
```

```json
{
  "id": "a1b2c3d4-...",
  "amount": 1000,
  "currency": "BRL",
  "status": "pending",
  "capture_method": "manual",
  "installments": 3,
  "refunded_amount": 0,
  "created_at": "2026-07-06T22:53:41Z"
}
```

**Capturar um pagamento autorizado** (fluxo `capture_method: manual`)

Após a autorização (`status: authorized`), a captura liquida os fundos:

```bash
curl -X POST http://localhost:8080/api/v1/payments/{id}/capture
# 200 → { ..., "status": "completed" }
# 409 → se o pagamento não estiver "authorized"
```

**Estornar um pagamento** (total ou parcial)

O corpo é opcional: sem `amount` (ou `amount` ausente) estorna o saldo restante por completo.

```bash
# Estorno parcial de 400 centavos
curl -X POST http://localhost:8080/api/v1/payments/{id}/refund \
  -H "Content-Type: application/json" \
  -d '{"amount": 400}'
# 200 → { ..., "status": "partially_refunded", "refunded_amount": 400 }

# Estorno total do restante
curl -X POST http://localhost:8080/api/v1/payments/{id}/refund
# 200 → { ..., "status": "refunded" }
# 400 → se o valor exceder o saldo estornável
# 409 → se o pagamento não estiver "completed"/"partially_refunded"
```

**Buscar por ID**

```bash
curl http://localhost:8080/api/v1/payments/{id}
```

**Listar (com paginação)**

```bash
curl "http://localhost:8080/api/v1/payments?page=1&limit=10&sort=created_at&order=desc&status=pending"
```

```json
{
  "data": [
    {
      "id": "a1b2c3d4-...",
      "amount": 1000,
      "currency": "BRL",
      "status": "pending",
      "created_at": "2026-07-06T22:53:41Z"
    }
  ],
  "page": "1",
  "limit": "10",
  "total": 25,
  "total_pages": 3
}
```

> `total` e `total_pages` refletem o **total de registros no banco** (com filtros aplicados), não apenas os itens da página atual.

**Registrar um webhook**

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{"url": "https://minha-loja.com/webhooks/pagamentos", "event_type": "payment.completed"}'
```

```json
{
  "id": "e5f6...",
  "url": "https://minha-loja.com/webhooks/pagamentos",
  "event_type": "payment.completed",
  "secret": "b1c2d3e4-...",
  "active": true,
  "created_at": "2026-07-07T18:00:00Z"
}
```

> Se `secret` não for informado, um é gerado automaticamente. **Guarde-o**: ele é usado para validar a assinatura HMAC de cada entrega.

Cada entrega enviada ao endpoint do lojista inclui os cabeçalhos:

| Cabeçalho | Descrição |
|---|---|
| `X-Webhook-Event` | Tipo do evento (ex.: `payment.completed`) |
| `X-Webhook-Id` | Id determinístico da entrega (dedup no lojista) |
| `X-Webhook-Signature` | `sha256=<hmac>` do corpo, calculado com o `secret` da assinatura |

Validação da assinatura no lado do lojista (exemplo em Go):

```go
mac := hmac.New(sha256.New, []byte(secret))
mac.Write(body)
expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
valid := hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Webhook-Signature")))
```

#### Query params da listagem

| Param | Padrão | Descrição |
|---|---|---|
| `page` | `1` | Página atual |
| `limit` | `10` | Itens por página |
| `sort` | `created_at` | Coluna de ordenação (`id`, `amount`, `currency`, `status`, `created_at`) |
| `order` | `desc` | Direção (`asc` ou `desc`) |
| `status` | — | Filtro opcional por status (`pending`, `authorized`, `completed`, `failed`, `refunded`, `partially_refunded`) |

---

## Idempotência

O endpoint `POST /api/v1/payments` exige o header **`Idempotency-Key`**. Requisições repetidas com a mesma chave e o mesmo body retornam a resposta original, sem criar pagamento duplicado.

### Comportamento

| Cenário | HTTP | Descrição |
|---|---|---|
| Primeira requisição com a key | `201` | Cria o pagamento e cacheia a resposta no Redis |
| Mesma key + mesmo body | `201` | Retorna a resposta cacheada |
| Mesma key + body diferente | `409` | `idempotency key already exists` |
| Requisição concorrente (lock ativo) | `409` | `request already processing` |
| Header ausente | `400` | `Idempotency-Key is required` |

### Exemplo — reenvio seguro

```bash
KEY=$(uuidgen)

# Primeira chamada — cria o pagamento
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $KEY" \
  -d '{"amount": 5000, "currency": "BRL"}'

# Segunda chamada — retorna o mesmo resultado (sem duplicata)
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $KEY" \
  -d '{"amount": 5000, "currency": "BRL"}'
```

### Configuração Redis

| Variável | Padrão | Descrição |
|---|---|---|
| `IDEMPOTENCY_TTL` | `24h` | Tempo de vida da resposta cacheada |
| `IDEMPOTENCY_LOCK_TTL` | `30s` | Tempo máximo do lock de processamento |

---

## Como rodar

### Pré-requisitos

- [Docker](https://docs.docker.com/get-docker/) e Docker Compose
- (Opcional) Go 1.26+ para rodar fora do Docker

### Desenvolvimento (com hot-reload)

```bash
cp .env.example .env
docker compose up
```

O arquivo `docker-compose.override.yml` é carregado automaticamente e configura:
- **Air** para recompilar ao salvar arquivos `.go`
- Volume montado com o código local
- Cache de módulos Go

```bash
# Ver logs da API
docker compose logs -f api

# Ver logs do relay do outbox
docker compose logs -f outbox

# Ver logs do consumer
docker compose logs -f consumer

# Ver logs do webhook service
docker compose logs -f webhook
```

> Se o build falhar com `error obtaining VCS status`, o `.air.toml` já inclui `-buildvcs=false` para contornar isso dentro do Docker.

> Ao alterar credenciais do PostgreSQL no `.env`, use `docker compose down -v` para recriar os volumes com os novos valores.

### Produção (build otimizado)

```bash
docker compose -f docker-compose.yml up --build -d
```

Usa o `Dockerfile` multi-stage (imagem Alpine enxuta), sem hot-reload.

### Rodar localmente (sem Docker na API)

```bash
# Subir Postgres, Redis e RabbitMQ
docker compose up postgres redis rabbitmq -d

# Configurar env vars
cp .env.example .env

# Aplicar migrations e popular dados (opcional)
make migrate-up
make seed

# Rodar a API
go run ./cmd/api

# Em outro terminal, rodar o relay do outbox (publica os eventos)
go run ./cmd/outbox

# Em outro terminal, rodar o consumer (processa os eventos)
go run ./cmd/consumer

# Em outro terminal, rodar o webhook service (entrega payment.completed)
go run ./cmd/webhook

# Em outro terminal, rodar o notification service (notifica o usuário final)
go run ./cmd/notification
```

---

## Testes

A suíte de testes cobre domínio, use cases, idempotência, handlers HTTP e adapters in-memory/Redis (via `miniredis`).

```bash
make test              # executa todos os testes
make test-coverage     # valida cobertura >= 80%
```

O `make test-coverage` mede os pacotes de negócio:

```
internal/application/...
internal/domain/...
internal/infrastructure/cache/...
internal/infrastructure/config/...
internal/infrastructure/http/...
internal/infrastructure/persistence/memory/...
internal/database/factory/...
```

Pacotes excluídos do cálculo (bootstrap ou dependem de infra externa):

- `cmd/` — entrypoints
- `internal/infrastructure/persistence/postgres` — requer PostgreSQL
- `internal/infrastructure/messaging/rabbitmq` — requer RabbitMQ
- `internal/database/migrate`, `seeder` — scripts operacionais

Relatório HTML da cobertura:

```bash
make test-coverage
go tool cover -html=coverage.out
```

---

## Migrations, factories e seeders

### Migrations

Schema gerenciado por **golang-migrate** em `db/migrations/`. A API executa `migrate up` automaticamente ao iniciar.

```bash
make migrate-up          # aplicar migrations
make migrate-down        # reverter tudo
make migrate-version     # ver versão atual
go run ./cmd/migrate steps -1  # reverter 1 migration
```

### Factories

Gera pagamentos fake para testes e seeders:

```go
factory := factory.NewPaymentFactory()

p := factory.Make()

p := factory.NewPaymentFactory().
    WithAmount(5000).
    WithCurrency("BRL").
    WithStatus(payment.StatusCompleted).
    Make()

payments := factory.NewPaymentFactory().MakeMany(10)
```

### Seeders

```bash
make seed                # insere SEED_COUNT pagamentos (padrão: 25)
make seed-fresh          # limpa a tabela e reinsere

go run ./cmd/seed -count=50
go run ./cmd/seed -count=25 -fresh
```

---

## Variáveis de ambiente

Copie `.env.example` para `.env` e ajuste conforme necessário.

| Variável | Padrão (.env.example) | Descrição |
|---|---|---|
| `PORT` | `8080` | Porta da API |
| `APP_PORT` | `8080` | Porta exposta no host (Docker) |
| `POSTGRES_HOST` | `localhost` | Host do PostgreSQL |
| `POSTGRES_PORT` | `5432` | Porta do PostgreSQL |
| `POSTGRES_USER` | `payment` | Usuário do banco |
| `POSTGRES_PASSWORD` | `payment` | Senha do banco |
| `POSTGRES_DB` | `payment_db` | Nome do banco |
| `REDIS_HOST` | `localhost` | Host do Redis |
| `REDIS_PORT` | `6379` | Porta do Redis |
| `IDEMPOTENCY_TTL` | `24h` | TTL da resposta idempotente no Redis |
| `IDEMPOTENCY_LOCK_TTL` | `30s` | TTL do lock de processamento |
| `RABBITMQ_HOST` | `localhost` | Host do RabbitMQ |
| `RABBITMQ_PORT` | `5672` | Porta AMQP |
| `RABBITMQ_MANAGEMENT_PORT` | `15672` | Porta do painel web |
| `RABBITMQ_USER` | `payment` | Usuário RabbitMQ |
| `RABBITMQ_PASSWORD` | `payment` | Senha RabbitMQ |
| `RABBITMQ_VHOST` | `/` | Virtual host |
| `RABBITMQ_EXCHANGE` | `payment.events` | Exchange de eventos |
| `RABBITMQ_QUEUE` | `payment` | Fila de pagamentos criados |
| `RABBITMQ_MAX_RETRIES` | `3` | Tentativas de processamento antes de mandar a mensagem à DLQ |
| `RABBITMQ_RETRY_DELAY` | `2s` | Espera entre as tentativas de processamento |
| `OUTBOX_POLL_INTERVAL` | `1s` | Intervalo de varredura do relay do outbox |
| `OUTBOX_BATCH_SIZE` | `100` | Máx. de eventos publicados por ciclo do relay |
| `WEBHOOK_QUEUE` | `webhook.payment` | Fila do webhook service (ligada a `payment.completed` e `payment.failed`) |
| `WEBHOOK_HTTP_TIMEOUT` | `5s` | Timeout do POST ao endpoint do lojista |
| `WEBHOOK_RETRY_MAX_ATTEMPTS` | `5` | Tentativas antes de marcar a entrega como `exhausted` |
| `WEBHOOK_RETRY_BASE_DELAY` | `30s` | Atraso base do backoff exponencial (30s, 1m, 2m, ...) |
| `WEBHOOK_RETRY_POLL_INTERVAL` | `10s` | Intervalo de varredura das entregas falhas pelo poller de retry |
| `WEBHOOK_RETRY_BATCH_SIZE` | `100` | Máx. de entregas reprocessadas por ciclo do poller |
| `PSP_MOCK_LATENCY` | `0` | Latência simulada da autorização no PSP mock (ex.: `200ms`) |
| `NOTIFICATION_QUEUE` | `notification.payment` | Fila do notification service (ligada a `payment.completed` e `payment.failed`) |
| `NOTIFICATION_CHANNEL` | `email` | Canal padrão da notificação (`email`/`sms`/`push` — mock) |
| `SEED_COUNT` | `25` | Quantidade padrão de registros no seeder |

> Dentro do Docker Compose, use os nomes dos serviços: `POSTGRES_HOST=postgres`, `REDIS_HOST=redis`, `RABBITMQ_HOST=rabbitmq`.

> O `config.Load()` usa `godotenv` para carregar o `.env`, mas **não sobrescreve** variáveis já definidas no ambiente (ex.: as do `docker-compose.yml`).

---

## Banco de dados

O schema é versionado em `db/migrations/`:

```sql
CREATE TABLE payments (
    id              UUID PRIMARY KEY,
    amount          BIGINT NOT NULL CHECK (amount > 0),
    currency        VARCHAR(3) NOT NULL,
    status          VARCHAR(20) NOT NULL,   -- pending | authorized | completed | failed | refunded | partially_refunded
    capture_method  VARCHAR(20) NOT NULL DEFAULT 'automatic', -- automatic | manual
    installments    INT NOT NULL DEFAULT 1, -- número de parcelas (1..12)
    refunded_amount BIGINT NOT NULL DEFAULT 0, -- total já estornado (centavos)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Outbox Pattern: eventos gravados na mesma transação do pagamento
CREATE TABLE outbox_events (
    id            UUID PRIMARY KEY,
    aggregate_id  UUID NOT NULL,          -- id do pagamento
    event_type    VARCHAR(50) NOT NULL,   -- "payment.created" / "payment.completed"
    payload       JSONB NOT NULL,         -- corpo do evento
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at  TIMESTAMPTZ             -- NULL = ainda não publicado
);

-- Webhooks: assinaturas de endpoints externos (lojistas)
CREATE TABLE webhook_subscriptions (
    id          UUID PRIMARY KEY,
    url         TEXT NOT NULL,
    secret      TEXT NOT NULL,          -- usado para assinar (HMAC) as entregas
    event_type  VARCHAR(50) NOT NULL,   -- ex.: "payment.completed"
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Webhooks: log de entregas (auditoria + base para reentrega/retry)
CREATE TABLE webhook_deliveries (
    id               UUID PRIMARY KEY,
    subscription_id  UUID NOT NULL REFERENCES webhook_subscriptions (id) ON DELETE CASCADE,
    event_id         TEXT NOT NULL,          -- id estável do evento (dedup no lojista)
    event_type       VARCHAR(50) NOT NULL,   -- tipo do evento (para reenviar no retry)
    payload          TEXT NOT NULL,          -- corpo enviado (para reenviar no retry)
    status           VARCHAR(20) NOT NULL,   -- pending | delivered | failed | exhausted
    attempts         INT NOT NULL DEFAULT 0,
    last_error       TEXT,
    next_attempt_at  TIMESTAMPTZ,            -- quando fica elegível ao próximo retry
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índice do poller de retry: entregas falhas já no prazo de nova tentativa
CREATE INDEX idx_webhook_deliveries_retry
    ON webhook_deliveries (next_attempt_at) WHERE status = 'failed';

-- Notificações ao usuário final (log/auditoria + dedup por id determinístico)
CREATE TABLE notifications (
    id          TEXT PRIMARY KEY,       -- hash(pagamento, evento, canal) → dedup
    payment_id  UUID NOT NULL,
    event_type  VARCHAR(50) NOT NULL,
    channel     VARCHAR(20) NOT NULL,   -- email | sms | push
    recipient   TEXT NOT NULL,
    message     TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL,   -- pending | sent | failed
    last_error  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Os dados ficam no volume Docker `postgres_data` e **persistem** entre restarts da API.

```bash
# Acessar o banco
docker exec -it payment_postgres psql -U payment -d payment_db

# Verificar migrations
docker exec -it payment_postgres psql -U payment -d payment_db -c "SELECT * FROM schema_migrations;"
```

---

## Docker

### Serviços

| Serviço | Container | Porta | Descrição |
|---|---|---|---|
| `api` | `payment_api` | 8080 | API Go |
| `outbox` | `payment_outbox` | — | Relay: publica eventos pendentes da tabela `outbox_events` |
| `consumer` | `payment_consumer` | — | Consome `payment.created`, autoriza no PSP e conclui/recusa pagamentos |
| `webhook` | `payment_webhook` | — | Consome `payment.completed`/`payment.failed`/`payment.refunded`, entrega webhooks assinados e reenvia falhas (retry com backoff) |
| `notification` | `payment_notification` | — | Consome `payment.completed`/`payment.failed`/`payment.refunded` e notifica o usuário final (mock via log) |
| `postgres` | `payment_postgres` | 5432 | Banco de dados |
| `redis` | `payment_redis` | 6379 | Idempotência |
| `rabbitmq` | `payment_rabbitmq` | 5672 / 15672 | Mensageria + painel web |

> Sempre suba a stack completa com `docker compose up -d` (sem nomear serviços) para garantir que `outbox`, `consumer` e `webhook` também iniciem. Sem o `outbox`, os eventos ficam presos na tabela `outbox_events` (nunca chegam ao broker); sem o `consumer`, ficam presos na fila. Nos dois casos o pagamento não sai de `pending`. O `webhook` só entrega notificações a lojistas quando há assinaturas cadastradas.

Painel RabbitMQ: [http://localhost:15672](http://localhost:15672) (credenciais conforme `.env`)

### Eventos publicados

| Evento | Exchange | Routing key | Payload |
|---|---|---|---|
| Pagamento criado | `payment.events` | `payment.created` | `{ id, amount, currency, status, created_at }` |
| Pagamento autorizado (captura manual) | `payment.events` | `payment.authorized` | `{ id, amount, currency, status, created_at }` |
| Pagamento concluído/capturado | `payment.events` | `payment.completed` | `{ id, amount, currency, status, created_at }` |
| Pagamento recusado | `payment.events` | `payment.failed` | `{ id, amount, currency, status, created_at }` |
| Pagamento estornado | `payment.events` | `payment.refunded` | `{ id, amount, currency, status, created_at }` |

### Comandos úteis

```bash
# Subir tudo (inclui o consumer)
docker compose up -d

# Conferir se outbox, consumer e webhook estão de pé
docker compose ps outbox consumer webhook

# Ver logs do relay, do consumer e do webhook
docker compose logs -f outbox consumer webhook

# Rebuild apenas a API
docker compose up -d --build api

# Parar tudo
docker compose down

# Parar e remover volumes (apaga dados!)
docker compose down -v
```

### Modos de build

| Arquivo | Uso | Hot-reload |
|---|---|---|
| `Dockerfile.dev` + `override` | Desenvolvimento local | Sim (Air) |
| `Dockerfile` | Produção | Não |

---

## Próximos passos

- [x] Consumer RabbitMQ (processar eventos assíncronos)
- [x] Outbox Pattern (publicação transacional de eventos)
- [x] Webhook Service (notificações assinadas a lojistas)
- [x] PSP Mock (autorização aprovada/recusada, base para falhas reais)
- [x] Retry de entregas de webhook (backoff exponencial + estado `exhausted`)
- [x] Retry de processamento no consumer + DLQ (DLX por fila, `<fila>.dlq`)
- [x] Notification Service (notifica o usuário final via canal mock)
- [ ] Reprocessamento automático da DLQ (hoje é manual pelo painel/CLI)
- [ ] Canal real de notificação (e-mail/SMS/push) no lugar do mock
- [ ] Use cases: falhar pagamento
- [ ] Endpoint DELETE exposto na API
- [ ] Testes de integração com PostgreSQL e RabbitMQ
- [ ] CI/CD pipeline

## Roadmap

✅ **Payment API** — API REST (Gin) para criar, consultar e listar pagamentos.

↓

✅ **Redis Idempotência** — evita cobrança duplicada: mesma `Idempotency-Key` retorna a resposta original.

↓

✅ **Rabbit Publisher** — publica eventos de pagamento (`payment.created`, etc.) no RabbitMQ.

↓

✅ **Rabbit Consumer** — consome os eventos e processa os pagamentos de forma assíncrona.

↓

✅ **Outbox Pattern** — grava evento e estado na mesma transação; um relay publica depois (sem perda de evento).

↓

✅ **Webhook Service** — notifica lojistas via POST assinado (HMAC) quando o pagamento é concluído/recusado.

↓

✅ **PSP Mock** — simula o provedor de pagamento (aprova/recusa + latência) para exercitar falhas reais.

↓

✅ **Retry (entregas de webhook)** — reenvia entregas que falharam com backoff exponencial e limite de tentativas.

↓

✅ **Dead Letter Queue** — isola mensagens que esgotaram as tentativas (DLX + `<fila>.dlq`) para análise, sem travar a fila nem perder mensagens.

↓

✅ **Notification Service** — envia notificações ao usuário final (e-mail/SMS/push — mock via log) a partir dos eventos.

↓

✅ **Payment Service** — evoluiu o núcleo de pagamentos: captura manual (`authorized` → `completed`), estornos total/parcial (`refunded`/`partially_refunded`) e parcelamento (`installments`).

↓

⬜ **Audit Service** — registra trilha de auditoria imutável de todas as ações e mudanças de estado.

↓

⬜ **Saga** — coordena transações distribuídas com passos compensatórios quando algo falha no meio.

↓

⬜ **OpenTelemetry** — instrumenta traces/métricas/logs para rastrear uma requisição ponta a ponta.

↓

⬜ **Grafana** — dashboards para visualizar métricas, logs e traces dos serviços.

↓

⬜ **Prometheus** — coleta e armazena métricas para alertas e monitoramento.

↓

⬜ **Kubernetes** — orquestra o deploy, escala e resiliência dos serviços em produção.