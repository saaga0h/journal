# journal-ingest-standing.hcl
# Daily WebDAV standing document ingestion + reassociation.
# Runs at 06:00 UTC every day.
#
# Force an immediate run (development/testing):
#   nomad job periodic force journal-ingest-standing

job "journal-ingest-standing" {
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


  periodic {
    crons            = ["0 6 * * *"]
    prohibit_overlap = true
    time_zone        = "UTC"
  }

  group "ingest" {
    restart {
      attempts = 2
      interval = "10m"
      delay    = "30s"
      mode     = "fail"
    }

    task "run" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args = [
          "-c",
          "chmod +x ${NOMAD_TASK_DIR}/ingest-webdav-standing ${NOMAD_TASK_DIR}/reassociate && ${NOMAD_TASK_DIR}/ingest-webdav-standing && ${NOMAD_TASK_DIR}/reassociate",
        ]
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/ingest-webdav-standing"
        destination = "local/ingest-webdav-standing"
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
MQTT_BROKER_URL={{ .Data.data.MQTT_BROKER_URL }}
MQTT_USER={{ .Data.data.MQTT_USER }}
MQTT_PASSWORD={{ .Data.data.MQTT_PASSWORD }}
OLLAMA_BASE_URL={{ .Data.data.OLLAMA_BASE_URL }}
OLLAMA_EMBED_MODEL={{ .Data.data.OLLAMA_EMBED_MODEL }}
OLLAMA_CHAT_MODEL={{ .Data.data.OLLAMA_CHAT_MODEL }}
OLLAMA_CHAT_NUM_CTX={{ .Data.data.OLLAMA_CHAT_NUM_CTX }}
WEBDAV_URL={{ .Data.data.WEBDAV_URL }}
WEBDAV_USERNAME={{ .Data.data.WEBDAV_USERNAME }}
WEBDAV_PASSWORD={{ .Data.data.WEBDAV_PASSWORD }}
WEBDAV_STANDING_PATH={{ .Data.data.WEBDAV_STANDING_PATH }}
WEBDAV_ENTRIES_PATH={{ .Data.data.WEBDAV_ENTRIES_PATH }}
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
    }
  }
}
