# SQLite to PostgreSQL + GORM Migration Plan

## Overview
Migrate from SQLite with raw SQL queries to PostgreSQL with GORM ORM.

## Benefits
1. **Better Performance** - PostgreSQL handles concurrent writes better
2. **Production Ready** - PostgreSQL is more suitable for production
3. **Type Safety** - GORM provides compile-time type checking
4. **Easier Queries** - GORM simplifies complex queries
5. **Migrations** - Built-in migration support
6. **Relationships** - Better handling of foreign keys and relationships

## Implementation Steps

### Phase 1: Setup GORM & PostgreSQL
1. Add GORM dependencies to `go.mod`
2. Add PostgreSQL driver
3. Create database models in `internal/models/`
4. Setup database connection with GORM

### Phase 2: Define Models
Create GORM models for:
- Messages [DONE]
- Contacts [DONE]
- Templates [DONE]
- Automation Rules [DONE]
- Automation Logs [DONE]
- Flows (Relational: Nodes & Edges) [DONE]
- Conversation Sessions [DONE]
- Media [DONE]

### Phase 3: Update Database Layer
1. Create new `internal/database/gorm.go` with GORM connection
2. Keep existing `db.go` for backward compatibility during migration
3. Implement auto-migration for all models

### Phase 4: Refactor Repositories
Create repository pattern:
- `internal/repositories/message_repo.go`
- `internal/repositories/contact_repo.go`
- `internal/repositories/flow_repo.go`
- `internal/repositories/session_repo.go`
- etc.

### Phase 5: Update Services
Update all services to use repositories instead of raw SQL

### Phase 6: Testing & Deployment
1. Test all endpoints [DONE]
2. Create migration script for existing data [DONE]
3. Sync PostgreSQL ID sequences [DONE]
4. Deploy to production

## File Structure
```
backend/
├── internal/
│   ├── models/          # GORM models
│   │   ├── message.go
│   │   ├── contact.go
│   │   ├── flow.go
│   │   ├── session.go
│   │   └── ...
│   ├── repositories/    # Data access layer
│   │   ├── message_repo.go
│   │   ├── contact_repo.go
│   │   └── ...
│   └── database/
│       ├── gorm.go      # GORM connection
│       └── db.go        # Legacy (to be removed)
```

## Environment Variables
Add to `.env`:
```
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=whatsapp_gateway
```

## Dependencies to Add
```
go get -u gorm.io/gorm
go get -u gorm.io/driver/postgres
```

## Migration Priority
1. **High Priority** (Core functionality):
   - Messages
   - Contacts
   - Flows
   - Conversation Sessions

2. **Medium Priority**:
   - Templates
   - Automation Rules
   - Media

3. **Low Priority**:
   - Logs
   - Analytics

## Rollback Plan
Keep SQLite code until PostgreSQL is fully tested and stable.
