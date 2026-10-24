# UI Rules

Short rules so new screens do not drift from the existing density.

## Sidebar

- Fixed width: 232px
- No collapse mode
- Brand is the only header content

## Brand

The in-app brand is just the word `Larder`, set in Space Grotesk 600, 18px. There is no logo mark and no collapse control in the app shell.

## Page toolbars

Every resource page uses the same structure:

```text
left:  search + filters
right: view controls + actions
```

Use `filterBar`, `filterLeft`, `filterRight`. The primary create action is always the last item on the right.

## Search

Always use `AssetSearch` for page-level search.

- 32px high
- 180px wide (170px in History)
- icon inside the field
- no page-specific search markup

Search fields inside catalog sheets are forms, not page filters, and may include a submit button.

## Buttons

- Page filter/control buttons: `size="sm"` (32px)
- Detail sheet inline actions: `size="xs"` (24px)
- Dialog footer actions: `size="sm"` (32px)
- Icon-only actions: `icon-xs` or `icon-sm`

Do not mix `sm` and `xs` inside one detail action row.

## Form inputs

Dialog and inline form inputs use 30px height and 11px text, overriding shadcn defaults.

## Dialogs

- Header: 16px title, 12px description
- Footer: border top, 32px buttons, actions right aligned
- Destructive confirmation remains a dialog, never inline

## Detail sheets

- Width 440px for resource details
- Sections separated by hairline borders
- Secondary actions are small outline buttons
- Destructive action is red but stays in the same action row

## Spacing

- Sidebar group vertical padding: 4px
- Page toolbar padding: 2px 2px 12px
- Dialog body padding: 16–24px
