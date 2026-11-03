package io.github.dennisdko.gotimerecording

/** placeName writes a choice of public holidays in words: a place as its country and region, none as the page says it. */
fun Language.placeName(place: String): String {
    if (place.isEmpty() || place == "none") return text("holiday.noneKept", "none")

    val country = place.substringBefore('-')
    if ('-' !in place) return countryName(country)

    return countryName(country) + ", " + regionName(place)
}

/** regionsCalled is what a country calls the parts its holidays differ by, as the page's picker names them. */
fun Language.regionsCalled(country: String): String = when (country) {
    "DE" -> text("holiday.region", "State")
    "GB" -> text("holiday.nation", "Nation")
    else -> text("holiday.area", "Region")
}

/** countryName is what a country holidays can be kept in is called, as the page calls it. */
fun Language.countryName(code: String): String = when (code) {
    "AT" -> text("country.AT", "Austria")
    "BE" -> text("country.BE", "Belgium")
    "CZ" -> text("country.CZ", "Czechia")
    "DE" -> text("country.DE", "Germany")
    "DK" -> text("country.DK", "Denmark")
    "FI" -> text("country.FI", "Finland")
    "FR" -> text("country.FR", "France")
    "GB" -> text("country.GB", "United Kingdom")
    "IE" -> text("country.IE", "Ireland")
    "IT" -> text("country.IT", "Italy")
    "LU" -> text("country.LU", "Luxembourg")
    "NL" -> text("country.NL", "Netherlands")
    "NO" -> text("country.NO", "Norway")
    "PL" -> text("country.PL", "Poland")
    "PT" -> text("country.PT", "Portugal")
    "SE" -> text("country.SE", "Sweden")
    "US" -> text("country.US", "United States")
    else -> code
}

/** regionName is what a region that keeps holidays of its own is called, by its place, as the page calls it. */
fun Language.regionName(place: String): String = when (place) {
    "DE-BW" -> text("region.DE-BW", "Baden-Wuerttemberg")
    "DE-BY" -> text("region.DE-BY", "Bavaria")
    "DE-BE" -> text("region.DE-BE", "Berlin")
    "DE-BB" -> text("region.DE-BB", "Brandenburg")
    "DE-HB" -> text("region.DE-HB", "Bremen")
    "DE-HH" -> text("region.DE-HH", "Hamburg")
    "DE-HE" -> text("region.DE-HE", "Hesse")
    "DE-MV" -> text("region.DE-MV", "Mecklenburg-Western Pomerania")
    "DE-NI" -> text("region.DE-NI", "Lower Saxony")
    "DE-NW" -> text("region.DE-NW", "North Rhine-Westphalia")
    "DE-RP" -> text("region.DE-RP", "Rhineland-Palatinate")
    "DE-SL" -> text("region.DE-SL", "Saarland")
    "DE-SN" -> text("region.DE-SN", "Saxony")
    "DE-ST" -> text("region.DE-ST", "Saxony-Anhalt")
    "DE-SH" -> text("region.DE-SH", "Schleswig-Holstein")
    "DE-TH" -> text("region.DE-TH", "Thuringia")
    "FR-6AE" -> text("region.FR-6AE", "Alsace")
    "FR-57" -> text("region.FR-57", "Moselle")
    "GB-ENG" -> text("region.GB-ENG", "England")
    "GB-WLS" -> text("region.GB-WLS", "Wales")
    "GB-SCT" -> text("region.GB-SCT", "Scotland")
    "GB-NIR" -> text("region.GB-NIR", "Northern Ireland")
    "IT-BZ" -> text("region.IT-BZ", "South Tyrol")
    "PT-20" -> text("region.PT-20", "Azores")
    "PT-30" -> text("region.PT-30", "Madeira")
    else -> place
}
