# secure-oci-base

Générateur minimal d'image OCI en Go, sans Dockerfile et sans dépendance Go tierce.

## Propriétés

- OCI Image Layout
- construction reproductible
- utilisateur non-root `65532:65532`
- aucun secret dans l'image
- métadonnées mTLS et TLS 1.2 minimum
- validation des chemins et permissions
- bibliothèque standard Go uniquement
- couverture unitaire actuelle : `100,0 %`
- seuil CI obligatoire : strictement supérieur à `99 %`

Les cgroups, namespaces réseau, seccomp et politiques réseau restent des responsabilités du runtime OCI et de l'orchestrateur.

## Tests

```bash
go test ./...
go test -race ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Le workflow GitHub Actions échoue automatiquement si la couverture totale tombe à `99 %` ou moins.

## Construire une image

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o service ./cmd/your-service
go run ./cmd/oci-builder -binary ./service -output ./oci-image -arch amd64
```