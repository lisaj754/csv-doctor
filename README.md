# csvdoctor

A command line tool that checks a CSV file for structural problems and
points at exactly where they are.

## Why

Most CSV tools either accept anything (silently mangling the data) or
reject broken input with a message like `csv: record on line 4: wrong
number of fields`, which tells you which row is bad but not which field,
which character, or why. When the file has 40,000 rows and a stray quote
somewhere, that's not enough to fix it quickly.

csvdoctor scans the file itself, byte by byte, and reports every problem
it finds with a line number, a column number, and the source line printed
with a caret under the exact character, the way a compiler error looks.
It reports all problems in one pass instead of stopping at the first one.

## Usage

```
$ csvdoctor orders.csv
$ csvdoctor --delimiter ';' orders.csv
$ csvdoctor --delimiter '\t' orders.tsv
```

The delimiter defaults to a comma. It must be a single character; pass
`\t` literally (backslash-t) to scan tab-separated files.

Given a file like this:

```
id,item,price,qty
1001,widget,9.99,3
1002,gadget,14.50
1003,thing"amajig,3.00,1
1004,"sprocket,7.25,2
```

csvdoctor prints:

```
orders.csv:3:1: row has 3 field(s), expected 4 (set by row 1)
    3 | 1002,gadget,14.50
      | ^
orders.csv:4:11: quote character inside an unquoted field; wrap the whole field in quotes if it needs to contain one
    4 | 1003,thing"amajig,3.00,1
      |           ^
orders.csv:5:6: quoted field is never closed (opening quote here has no matching closing quote before end of file)
    5 | 1004,"sprocket,7.25,2
      |      ^
```

exit status is 0 when the file is clean and 1 when problems were found,
so it works as a CI check.

## What it checks

- every row has the same number of fields as the first row
- a quote character appearing inside an unquoted field
- text appearing after a quoted field's closing quote but before the next
  delimiter or newline
- a quoted field that is never closed before end of file

Quoted fields containing commas, newlines, and escaped (`""`) quotes are
handled correctly and are not flagged.

## Building

```
go build -o csvdoctor .
```

No dependencies outside the standard library.

## Status

Early. The field-count baseline always comes from the first row; see the
issues for planned flags.
