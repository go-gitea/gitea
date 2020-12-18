# Inflection

Inflection pluralizes and singularizes English nouns

[![wercker status](https://app.wercker.com/status/f8c7432b097d1f4ce636879670be0930/s/master "wercker status")](https://app.wercker.com/project/byKey/f8c7432b097d1f4ce636879670be0930)

## Basic Usage

```go
inflection.Plural("person") => "people"
inflection.Plural("Person") => "People"
inflection.Plural("PERSON") => "PEOPLE"
inflection.Plural("bus")    => "buses"
inflection.Plural("BUS")    => "BUSES"
inflection.Plural("Bus")    => "Buses"

inflection.Singular("people") => "person"
inflection.Singular("People") => "Person"
inflection.Singular("PEOPLE") => "PERSON"
inflection.Singular("buses")  => "bus"
inflection.Singular("BUSES")  => "BUS"
inflection.Singular("Buses")  => "Bus"

inflection.Plural("FancyPerson") => "FancyPeople"
inflection.Singular("FancyPeople") => "FancyPerson"
```

## Register Rules

Standard rules are from Rails's ActiveSupport (https://github.com/rails/rails/blob/master/activesupport/lib/active_support/inflections.rb)

If you want to register more rules, follow:

```
inflection.AddUncountable("fish")
inflection.AddIrregular("person", "people")
inflection.AddPlural("(bu)s$", "${1}ses") # "bus" => "buses" / "BUS" => "BUSES" / "Bus" => "Buses"
inflection.AddSingular("(bus)(es)?$", "${1}") # "buses" => "bus" / "Buses" => "Bus" / "BUSES" => "BUS"
```

## Contributing

You can help to make the project better, check out [http://gorm.io/contribute.html](http://gorm.io/contribute.html) for things you can do.

## Author

**jinzhu**

* <http://github.com/jinzhu>
* <wosmvp@gmail.com>
* <http://twitter.com/zhangjinzhu>

## License

Released under the [MIT License](http://www.opensource.org/licenses/MIT).
