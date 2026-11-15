# Food seed provenance

`foods.csv` holds the global foods loaded at startup. Values are per 100 g of
edible portion.

## Source

Every row was read on 2026-11-16 from the online Indonesian Food Composition
Data (Data Komposisi Pangan Indonesia, the online edition of TKPI) at
<https://www.panganku.org>. The `source` column carries the TKPI food code and
the exact TKPI food name, so each row can be looked up again.

## What `verified` means here

`verified` means the row matches what panganku.org displayed for that code on
the date above. It does not mean the values were checked against the printed
TKPI book.

## Known precision limit

The website displays protein, fat and carbohydrate in whole grams, and the
figures look truncated rather than rounded (for example, protein of fresh
spinach is shown as 0 g). Energy is shown in whole kcal. The seed stores
exactly what the source shows instead of guessing the missing decimals, so
macros can be up to 1 g per 100 g lower than the printed table.

To raise the precision, replace the macro columns with the values from the
printed TKPI and update the `source` column to say so.

## Adding rows

1. Find the food on panganku.org and note its code.
2. Copy energy, protein, carbohydrate and fat per 100 g as displayed.
3. Use `verified` only if you copied the values from the source yourself;
   otherwise use `needs_verification`.

Seeding only inserts missing rows. Changing an existing row in the CSV does
not update databases that were already seeded.
