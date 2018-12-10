package envstruct

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
)

var ReportWriter io.Writer = os.Stdout

// WriteReport will take a struct that is setup for envstruct and print
// out a report containing the struct field name, field type, environment
// variable for that field, whether or not the field is required and
// the value of that field. The report is written to `ReportWriter`
// which defaults to `os.StdOut`. Sensetive values that you would not
// want appearing in logs can be omitted with the `noreport` value in
// the `env` struct tag.
func WriteReport(t interface{}) error {
	w := tabwriter.NewWriter(ReportWriter, 0, 8, 2, ' ', 0)

	fmt.Fprintln(w, "FIELD NAME:\tTYPE:\tENV:\tREQUIRED:\tVALUE:")

	val := reflect.ValueOf(t).Elem()
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tag := typeField.Tag

		tagProperties := separateOnComma(tag.Get("env"))
		envVar := strings.ToUpper(tagProperties[indexEnvVar])
		isRequired := tagPropertiesContains(tagProperties, tagRequired)

		var displayedValue interface{} = valueField
		if tagPropertiesContains(tagProperties, tagNoReport) {
			displayedValue = "(OMITTED)"
		}

		fmt.Fprintln(w, fmt.Sprintf(
			"%v\t%v\t%v\t%t\t%v",
			typeField.Name,
			valueField.Type(),
			envVar,
			isRequired,
			displayedValue))
	}

	return w.Flush()
}
