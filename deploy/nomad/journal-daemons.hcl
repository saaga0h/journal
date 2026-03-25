# journal-daemons.hcl
# Long-running Journal services: entry-ingest and brief-assemble.
#
# Prerequisites:
#   - Vault policy "journal" applied (deploy/vault/journal-policy.hcl)
#   - raw_exec enabled on Nomad client:
#       plugin "raw_exec" { config { enabled = true } }
#   - enable_script_checks = true on Nomad client (for Consul health checks)
#   - Consul agent running on Nomad client
#
# Deploy:
#   nomad job run deploy/nomad/journal-daemons.hcl
#

job "journal-daemons" {
  datacenters = ["the-collective"]
  type        = "service"

  meta {
    artifact_base = "${ARTIFACT_BASE}"
  }

  constraint {
    attribute = "${meta.gpu}"
    operator  = "!="
    value     = "true"
  }


  group "daemons" {
    count = 1

    restart {
      attempts = 5
      interval = "5m"
      delay    = "15s"
      mode     = "delay"
    }

    # ── entry-ingest ──────────────────────────────────────────────────────────

    task "entry-ingest" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "chmod +x ${NOMAD_TASK_DIR}/entry-ingest && exec ${NOMAD_TASK_DIR}/entry-ingest"]
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/entry-ingest"
        destination = "local/entry-ingest"
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
MQTT_BROKER_URL={{ .Data.data.MQTT_BROKER_URL }}
MQTT_USER={{ .Data.data.MQTT_USER }}
MQTT_PASSWORD={{ .Data.data.MQTT_PASSWORD }}
OLLAMA_BASE_URL={{ .Data.data.OLLAMA_BASE_URL }}
OLLAMA_EMBED_MODEL={{ .Data.data.OLLAMA_EMBED_MODEL }}
OLLAMA_CHAT_MODEL={{ .Data.data.OLLAMA_CHAT_MODEL }}
OLLAMA_CHAT_NUM_CTX={{ .Data.data.OLLAMA_CHAT_NUM_CTX }}
ASSOCIATION_THRESHOLD={{ .Data.data.ASSOCIATION_THRESHOLD }}
{{ end }}
EOT
      }

      vault {
        policies = ["journal"]
      }

      resources {
        cpu    = 200
        memory = 128
      }

      service {
        name = "journal-entry-ingest"
        tags = ["journal"]

        check {
          type     = "script"
          name     = "process-alive"
          command  = "/bin/sh"
          args     = ["-c", "pgrep -x entry-ingest > /dev/null"]
          interval = "30s"
          timeout  = "5s"
        }
      }
    }

    # ── brief-assemble ────────────────────────────────────────────────────────

    task "brief-assemble" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "chmod +x ${NOMAD_TASK_DIR}/brief-assemble && exec ${NOMAD_TASK_DIR}/brief-assemble"]
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/brief-assemble"
        destination = "local/brief-assemble"
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
MQTT_BROKER_URL={{ .Data.data.MQTT_BROKER_URL }}
MQTT_USER={{ .Data.data.MQTT_USER }}
MQTT_PASSWORD={{ .Data.data.MQTT_PASSWORD }}
OLLAMA_BASE_URL={{ .Data.data.OLLAMA_BASE_URL }}
OLLAMA_EMBED_MODEL={{ .Data.data.OLLAMA_EMBED_MODEL }}
BRIEF_RELEVANCE_THRESHOLD={{ .Data.data.BRIEF_RELEVANCE_THRESHOLD }}
{{ end }}
EOT
      }

      vault {
        policies = ["journal"]
      }

      resources {
        cpu    = 100
        memory = 64
      }

      service {
        name = "journal-brief-assemble"
        tags = ["journal"]

        check {
          type     = "script"
          name     = "process-alive"
          command  = "/bin/sh"
          args     = ["-c", "pgrep -x brief-assemble > /dev/null"]
          interval = "30s"
          timeout  = "5s"
        }
      }
    }
  }
}
