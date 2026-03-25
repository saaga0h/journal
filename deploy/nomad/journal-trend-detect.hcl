# journal-trend-detect.hcl
# Daily trend detection: computes gravity profile and soul speed, publishes to MQTT.
# Runs at 06:30 UTC (after standing + entries ingest).
#
# Force an immediate run (development/testing):
#   nomad job periodic force journal-trend-detect

job "journal-trend-detect" {
  datacenters = ["the-collective"]
  type        = "batch"

  meta {
    artifact_base = "ARTIFACT_BASE_PLACEHOLDER"
  }

  constraint {
    attribute = "${meta.gpu}"
    operator  = "!="
    value     = "true"
  }


  periodic {
    crons            = ["30 6 * * *"]
    prohibit_overlap = true
    time_zone        = "UTC"
  }

  group "detect" {
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
        args    = ["-c", "chmod +x ${NOMAD_TASK_DIR}/trend-detect && exec ${NOMAD_TASK_DIR}/trend-detect"]
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/trend-detect"
        destination = "local/trend-detect"
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
ASSOCIATION_THRESHOLD={{ .Data.data.ASSOCIATION_THRESHOLD }}
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
    }
  }
}
