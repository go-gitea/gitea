# Markdown

## Text

### Inline elements

Text with **bold**, *italic*, ~~strikethrough~~, `inline code`, [a link](https://gitea.com), an autolink https://gitea.com and <kbd>Ctrl</kbd>+<kbd>C</kbd>.

H<sub>2</sub>O, x<sup>2</sup>, <mark>marked</mark>, <ins>inserted</ins>, <abbr title="HyperText Markup Language">HTML</abbr>, <samp>sample output</samp>, <var>variable</var>, <small>small</small>, <q>quote</q> and emoji :+1: :tada:.

### Paragraphs

A paragraph with a soft line break
continues on the same line, while a hard break\
starts a new line.

A second paragraph that is long enough to wrap onto multiple lines, so that the line height of paragraphs can be compared with the line height of list items further below.

### Headings

# Heading 1 with `code`
## Heading 2 with `code`
### Heading 3 with `code`
#### Heading 4 with `code`
##### Heading 5 with `code`
###### Heading 6 with `code`

### Horizontal rule

Text above the rule.

---

Text below the rule.

## Quotes

### Blockquote

> A blockquote with a paragraph.
>
> > A nested blockquote.
>
> - A list in a blockquote
> - Another item
>
> ```
> code in a blockquote
> ```

### Alerts

> [!NOTE]
> Useful information that users should know, even when skimming content.

> [!TIP]
> Helpful advice for doing things better or more easily.

> [!IMPORTANT]
> Key information users need to know to achieve their goal.

> [!WARNING]
> Urgent info that needs immediate user attention to avoid problems.

> [!CAUTION]
> Advises about risks or negative outcomes of certain actions.
>
> A second paragraph in an alert.

## Lists

### Tight list

- Item one
- Item two
  - Nested item
    - Deeper nested item
- Item three

### Loose list

- Item one

- Item two with a second paragraph

  The second paragraph.

- Item three

### Ordered list

1. Item one
2. Item two
    1. Nested item
        1. Deeper nested item
3. Item three

### Ordered list with type

<ol>
<li>Item one
<ol type="a">
<li>Nested item with <code>type="a"</code></li>
<li>Another nested item</li>
</ol>
</li>
</ol>

### Ordered list with start

5. Item five
6. Item six

### Code block in list

1. A tight item with a code block:
    ```js
    const a = 1;
    ```
2. Item after the code block

### Task list

- [ ] Unchecked task
- [x] Checked task
  - [ ] Nested task
- Normal item

1. [ ] Ordered task
2. [x] Ordered checked task

### Loose task list

- [ ] Loose task

- [x] Loose checked task

  A second paragraph in a loose task.

### Right-to-left task list

- [ ] משימה

- [x] משימה שהושלמה

## Code

### Code blocks

```go
package main

func main() {
  println("hello")
}
```

```
A plain code block with a long line that needs to scroll horizontally: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

    An indented code block

### Math

Inline math $E = mc^2$ in a sentence.

$$
\sum_{i=1}^n x_i
$$

## Tables

### Table

| Left | Center | Right |
| :--- | :---: | ---: |
| `code` | 1 | 1.00 |
| text | 22 | 22.00 |
| longer cell content | 333 | 333.00 |
| <img src="/assets/img/logo.svg" alt="logo" width="24"> image | 4444 | 4444.00 |

### Table with paragraphs

<table>
<tr><th>Header</th><th>Header</th></tr>
<tr><td><p>Paragraph in a cell</p></td><td>Text in a cell</td></tr>
</table>

## Other

### Details

<details>
<summary>Closed details</summary>

Hidden content.

</details>

<details open>
<summary>Open details</summary>

Visible content.

</details>

<details>
<summary><h3>Heading in summary</h3></summary>

Hidden content.

</details>

### Definition list

<dl>
<dt>Term</dt>
<dd>Definition</dd>
<dt>First term</dt>
<dt>Second term</dt>
<dd>Shared definition</dd>
</dl>

### Images

An inline image ![logo](/assets/img/logo.svg) in text.

<p align="center"><img src="/assets/img/logo.svg" alt="centered logo" width="64"></p>

### Footnotes

A sentence with a footnote[^1] and another one[^2][^3].

[^1]: The first footnote.
[^2]: The second footnote with **bold** text.
[^3]: The third footnote.
