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

O **Payment Service** expõe uma API HTTP para criar, consultar e listar pagamentos. Cada pagamento possui valor (`amount`), moeda (`currency`) e status (`pending`, `completed`, `failed`).

Principais capacidades:

- **Persistência** no PostgreSQL (adapter `pgx`)
- **Idempotência** no `POST /payments` via Redis (header `Idempotency-Key`)
- **Eventos** publicados no RabbitMQ ao criar um pagamento (`payment.created`)
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
| **RabbitMQ 3.13** | Publicação de eventos (`payment.created`) |
| **amqp091-go** | Cliente AMQP |
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
│   ├── migrate/main.go                      # CLI de migrations
│   └── seed/main.go                         # CLI de seeders
├── db/migrations/                           # SQL versionado (golang-migrate)
├── internal/
│   ├── domain/payment/                      # Núcleo do domínio
│   │   ├── payment.go                       # Entidade
│   │   ├── money.go                         # Value object
│   │   ├── status.go                        # Value object
│   │   ├── page.go                          # Resultado paginado
│   │   ├── publisher.go                     # Porta EventPublisher
│   │   ├── errors.go
│   │   └── repository.go                    # Porta Repository
│   ├── application/
│   │   ├── dto/                             # Objetos de transferência
│   │   ├── idempotency/                     # Serviço e contratos de idempotência
│   │   ├── payment/                         # Mapper e validator
│   │   └── usecase/                         # Casos de uso
│   ├── database/
│   │   ├── migrate/                         # Runner de migrations
│   │   ├── factory/                         # Factories de dados fake
│   │   └── seeder/                          # Seeders
│   ├── testutil/                            # Mocks compartilhados nos testes
│   └── infrastructure/
│       ├── config/
│       ├── cache/redis/                     # Cliente Redis + idempotência
│       ├── http/                            # Router, handlers, middleware
│       ├── messaging/rabbitmq/              # Adapter RabbitMQ (eventos)
│       └── persistence/
│           ├── postgres/                    # Adapter PostgreSQL (produção)
│           └── memory/                      # Adapter in-memory (testes)
├── docker-compose.yml                       # Orquestração (produção)
├── docker-compose.override.yml              # Hot-reload (dev, auto-carregado)
├── Dockerfile                               # Build de produção
├── Dockerfile.dev                           # Build de desenvolvimento (Air)
├── .air.toml                                # Configuração do hot-reload
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
    participant Repo as PostgresRepository
    participant RMQ as RabbitMQ
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
        UC->>Repo: Save(ctx, payment)
        Repo->>DB: INSERT INTO payments
        UC->>RMQ: PublishCreated(payment)
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

### Infraestrutura Docker

```mermaid
flowchart LR
    subgraph Host["Máquina local"]
        Client["Cliente\n(curl / Postman)"]
    end

    subgraph Docker["Docker Compose"]
        API["payment_api\n(Go + Air)"]
        PG["payment_postgres\n(PostgreSQL 16)"]
        RD["payment_redis\n(Redis 7)"]
        RMQ["payment_rabbitmq\n(RabbitMQ 3.13)"]
    end

    Client -->|:8080| API
    API -->|:5432| PG
    API -->|:6379| RD
    API -->|:5672| RMQ

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

### Exemplos

**Health check**

```bash
curl http://localhost:8080/ping
# {"message":"pong"}
```

**Criar pagamento**

```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"amount": 1000, "currency": "BRL"}'
```

```json
{
  "id": "a1b2c3d4-...",
  "amount": 1000,
  "currency": "BRL",
  "status": "pending",
  "created_at": "2026-07-06T22:53:41Z"
}
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

#### Query params da listagem

| Param | Padrão | Descrição |
|---|---|---|
| `page` | `1` | Página atual |
| `limit` | `10` | Itens por página |
| `sort` | `created_at` | Coluna de ordenação (`id`, `amount`, `currency`, `status`, `created_at`) |
| `order` | `desc` | Direção (`asc` ou `desc`) |
| `status` | — | Filtro opcional por status (`pending`, `completed`, `failed`) |

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
| `POSTGRES_USER` | `wallet` | Usuário do banco |
| `POSTGRES_PASSWORD` | `wallet` | Senha do banco |
| `POSTGRES_DB` | `wallet_db` | Nome do banco |
| `REDIS_HOST` | `localhost` | Host do Redis |
| `REDIS_PORT` | `6379` | Porta do Redis |
| `IDEMPOTENCY_TTL` | `24h` | TTL da resposta idempotente no Redis |
| `IDEMPOTENCY_LOCK_TTL` | `30s` | TTL do lock de processamento |
| `RABBITMQ_HOST` | `localhost` | Host do RabbitMQ |
| `RABBITMQ_PORT` | `5672` | Porta AMQP |
| `RABBITMQ_MANAGEMENT_PORT` | `15672` | Porta do painel web |
| `RABBITMQ_USER` | `wallet` | Usuário RabbitMQ |
| `RABBITMQ_PASSWORD` | `wallet` | Senha RabbitMQ |
| `RABBITMQ_VHOST` | `/` | Virtual host |
| `RABBITMQ_EXCHANGE` | `wallet.events` | Exchange de eventos |
| `RABBITMQ_QUEUE` | `wallet` | Fila de pagamentos criados |
| `SEED_COUNT` | `25` | Quantidade padrão de registros no seeder |

> Dentro do Docker Compose, use os nomes dos serviços: `POSTGRES_HOST=postgres`, `REDIS_HOST=redis`, `RABBITMQ_HOST=rabbitmq`.

> O `config.Load()` usa `godotenv` para carregar o `.env`, mas **não sobrescreve** variáveis já definidas no ambiente (ex.: as do `docker-compose.yml`).

---

## Banco de dados

O schema é versionado em `db/migrations/`:

```sql
CREATE TABLE payments (
    id          UUID PRIMARY KEY,
    amount      BIGINT NOT NULL CHECK (amount > 0),
    currency    VARCHAR(3) NOT NULL,
    status      VARCHAR(20) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Os dados ficam no volume Docker `postgres_data` e **persistem** entre restarts da API.

```bash
# Acessar o banco
docker exec -it payment_postgres psql -U wallet -d wallet_db

# Verificar migrations
docker exec -it payment_postgres psql -U wallet -d wallet_db -c "SELECT * FROM schema_migrations;"
```

---

## Docker

### Serviços

| Serviço | Container | Porta | Descrição |
|---|---|---|---|
| `api` | `payment_api` | 8080 | API Go |
| `postgres` | `payment_postgres` | 5432 | Banco de dados |
| `redis` | `payment_redis` | 6379 | Idempotência |
| `rabbitmq` | `payment_rabbitmq` | 5672 / 15672 | Mensageria + painel web |

Painel RabbitMQ: [http://localhost:15672](http://localhost:15672) (credenciais conforme `.env`)

### Eventos publicados

| Evento | Exchange | Routing key | Payload |
|---|---|---|---|
| Pagamento criado | `wallet.events` | `payment.created` | `{ id, amount, currency, status, created_at }` |

### Comandos úteis

```bash
# Subir tudo
docker compose up -d

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

- [ ] Consumer RabbitMQ (processar eventos assíncronos)
- [ ] Use cases: completar/falhar pagamento
- [ ] Endpoint DELETE exposto na API
- [ ] Testes de integração com PostgreSQL e RabbitMQ
- [ ] CI/CD pipeline

## Roadmap

✅ Payment API

↓

✅ Redis Idempotência

↓

✅ Rabbit Publisher

↓

⬜ Rabbit Consumer

↓

⬜ Outbox Pattern

↓

⬜ Webhook Service

↓

⬜ PSP Mock

↓

⬜ Retry

↓

⬜ Dead Letter Queue

↓

⬜ Notification Service

↓

⬜ Wallet Service

↓

⬜ Audit Service

↓

⬜ Saga

↓

⬜ OpenTelemetry

↓

⬜ Grafana

↓

⬜ Prometheus

↓

⬜ Kubernetes