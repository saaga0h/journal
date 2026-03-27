# journal-reembed.hcl
# One-shot re-embedding job. Registered automatically on deploy but only
# runs when explicitly dispatched — use after a new embedding model or
# after migration 009 (768→4096) wipes existing embeddings.
#
# Dispatch:
#   nomad job dispatch journal-reembed
#
# Sequence: reembed --force (all entries) → reassociate (recompute all associations)

job "journal-reembed" {
  datacenters = ["the-collective"]
  type        = "batch"

  meta {
    artifact_base = "${ARTIFACT_BASE}"
  }

  constraint {
    attribute = "${meta.gpu}"
    operator  = "!="
    value     = "true"
  }

  parameterized {
    payload = "forbidden"
  }

  group "reembed" {
    restart {
      attempts = 1
      interval = "30m"
      delay    = "30s"
      mode     = "fail"
    }

    task "run" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args = [
          "-c",
          "chmod +x ${NOMAD_TASK_DIR}/reembed ${NOMAD_TASK_DIR}/reassociate && ${NOMAD_TASK_DIR}/reembed --force && ${NOMAD_TASK_DIR}/reassociate",
        ]
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/reembed"
        destination = "local/reembed"
        mode        = "file"
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/reassociate"
        destination = "local/reassociate"
        mode        = "file"
      }

      template {
        destination = "secrets/journal.env"
        env         = true
        data        = <<EOT
{{ with secret "secret/data/nomad/journal" }}
DB_HOST={{ .Data.data.DB_HOST }}
DB_PORT={{ .Data.data.DB_PORT }}
DB_USER={{ .Data.data.DB_USER }}
DB_PASSWORD={{ .Data.data.DB_PASSWORD }}
DB_NAME={{ .Data.data.DB_NAME }}
DB_SSLMODE={{ .Data.data.DB_SSLMODE }}
OLLAMA_BASE_URL={{ .Data.data.OLLAMA_BASE_URL }}
OLLAMA_EMBED_MODEL={{ .Data.data.OLLAMA_EMBED_MODEL }}
ASSOCIATION_THRESHOLD={{ .Data.data.ASSOCIATION_THRESHOLD }}
{{ end }}
EOT
      }

      vault {
        policies = ["journal"]
      }

      resources {
        cpu    = 200
        memory = 256
      }
    }
  }
}
