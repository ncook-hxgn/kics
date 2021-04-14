package generic.openapi

checkOpenAPI(doc) = version {
  object.get(doc, "openapi", "undefined") != "undefined"
  version = doc.openapi
  regex.match("^3\\.0\\.\\d+$", doc.openapi)
} else = version {
  version = "undefined"
}
