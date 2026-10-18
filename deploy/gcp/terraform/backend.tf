terraform {
  backend "gcs" {
    bucket = "tfstatebucket10182026"
    prefix = "g21/terraform"
  }
}
