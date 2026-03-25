# journal-brief-trigger.hcl
# Daily morning brief trigger: publishes to journal/brief/trigger at 07:00 UTC.
# brief-assemble daemon (in journal-daemons.hcl) picks it up.
#
# Force an immediate run (development/testing):
#   nomad job periodic force journal-brief-trigger

job "journal-brief-trigger" {
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
    crons            = ["0 7 * * *"]
    prohibit_overlap = true
    time_zone        = "UTC"
  }

  group "trigger" {
    restart {
      attempts = 3
      interval = "5m"
      delay    = "10s"
      mode     = "fail"
    }

    task "run" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "chmod +x ${NOMAD_TASK_DIR}/brief-trigger && exec ${NOMAD_TASK_DIR}/brief-trigger"]
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/brief-trigger"
        destination = "local/brief-trigger"
        mode        = "file"
      }

      template {
        destination = "secrets/journal.env"
        env         = true
        data        = <<EOT
{{ with secret "secret/data/nomad/journal" }}
MQTT_BROKER_URL={{ .Data.data.MQTT_BROKER_URL }}
MQTT_USER={{ .Data.data.MQTT_USER }}
MQTT_PASSWORD={{ .Data.data.MQTT_PASSWORD }}
{{ end }}
EOT
      }

      vault {
        policies = ["journal"]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}
