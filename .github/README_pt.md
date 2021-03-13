<p align="center">
  <a href="https://gofiber.io">
    <img alt="Fiber" height="125" src="https://raw.githubusercontent.com/gofiber/docs/master/static/fiber_v2_logo.svg">
  </a>
  <br>
  <!-- base64 flags are available at https://www.phoca.cz/cssflags/ -->
   <a href="https://github.com/gofiber/fiber/blob/master/.github/README.md">
    <img height="20px" src="https://img.shields.io/badge/EN-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTIwMCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiB2aWV3Qm94PSIwIDAgNjAgMzAiIGhlaWdodD0iNjAwIj4NCjxkZWZzPg0KPGNsaXBQYXRoIGlkPSJ0Ij4NCjxwYXRoIGQ9Im0zMCwxNWgzMHYxNXp2MTVoLTMwemgtMzB2LTE1enYtMTVoMzB6Ii8+DQo8L2NsaXBQYXRoPg0KPC9kZWZzPg0KPHBhdGggZmlsbD0iIzAwMjQ3ZCIgZD0ibTAsMHYzMGg2MHYtMzB6Ii8+DQo8cGF0aCBzdHJva2U9IiNmZmYiIHN0cm9rZS13aWR0aD0iNiIgZD0ibTAsMGw2MCwzMG0wLTMwbC02MCwzMCIvPg0KPHBhdGggc3Ryb2tlPSIjY2YxNDJiIiBzdHJva2Utd2lkdGg9IjQiIGQ9Im0wLDBsNjAsMzBtMC0zMGwtNjAsMzAiIGNsaXAtcGF0aD0idXJsKCN0KSIvPg0KPHBhdGggc3Ryb2tlPSIjZmZmIiBzdHJva2Utd2lkdGg9IjEwIiBkPSJtMzAsMHYzMG0tMzAtMTVoNjAiLz4NCjxwYXRoIHN0cm9rZT0iI2NmMTQyYiIgc3Ryb2tlLXdpZHRoPSI2IiBkPSJtMzAsMHYzMG0tMzAtMTVoNjAiLz4NCjwvc3ZnPg0K">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ru.md">
    <img height="20px" src="https://img.shields.io/badge/RU-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA0NTAgMzAwIj4NCjxwYXRoIGZpbGw9IiNmZmYiIGQ9Im0wLDBoNDUwdjEwMGgtNDUweiIvPg0KPHBhdGggZmlsbD0iIzAwZiIgZD0ibTAsMTAwaDQ1MHYxMDBoLTQ1MHoiLz4NCjxwYXRoIGZpbGw9IiNmMDAiIGQ9Im0wLDIwMGg0NTB2MTAwaC00NTB6Ii8+DQo8L3N2Zz4NCg==">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_es.md">
    <img height="20px" src="https://img.shields.io/badge/ES-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA3NTAgNTAwIj4NCjxwYXRoIGZpbGw9IiNjNjBiMWUiIGQ9Im0wLDBoNzUwdjUwMGgtNzUweiIvPg0KPHBhdGggZmlsbD0iI2ZmYzQwMCIgZD0ibTAsMTI1aDc1MHYyNTBoLTc1MHoiLz4NCjwvc3ZnPg0K">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ja.md">
    <img height="20px" src="https://img.shields.io/badge/JA-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA5MDAgNjAwIj4NCjxwYXRoIGZpbGw9IiNmZmYiIGQ9Im0wLDBoOTAwdjYwMGgtOTAweiIvPg0KPGNpcmNsZSBmaWxsPSIjYmUwMDI2IiBjeD0iNDUwIiBjeT0iMzAwIiByPSIxODAiLz4NCjwvc3ZnPg0K">
  </a>
  <!-- <a href="https://github.com/gofiber/fiber/blob/master/.github/README_pt.md">
    <img height="20px" src="https://img.shields.io/badge/PT-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iaXNvLTg4NTktMSI/Pgo8IS0tIEdlbmVyYXRvcjogQWRvYmUgSWxsdXN0cmF0b3IgMTkuMC4wLCBTVkcgRXhwb3J0IFBsdWctSW4gLiBTVkcgVmVyc2lvbjogNi4wMCBCdWlsZCAwKSAgLS0+CjxzdmcgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgdmVyc2lvbj0iMS4xIiBpZD0iQ2FwYV8xIiB4PSIwcHgiIHk9IjBweCIgdmlld0JveD0iMCAwIDUxMiA1MTIiIHN0eWxlPSJlbmFibGUtYmFja2dyb3VuZDpuZXcgMCAwIDUxMiA1MTI7IiB4bWw6c3BhY2U9InByZXNlcnZlIj4KPHJlY3QgeT0iODUuMzM3IiBzdHlsZT0iZmlsbDojRDgwMDI3OyIgd2lkdGg9IjUxMiIgaGVpZ2h0PSIzNDEuMzI2Ii8+Cjxwb2x5Z29uIHN0eWxlPSJmaWxsOiM2REE1NDQ7IiBwb2ludHM9IjE5Ni42NDEsODUuMzM3IDE5Ni42NDEsMjYxLjU2NSAxOTYuNjQxLDQyNi42NjMgMCw0MjYuNjYzIDAsODUuMzM3ICIvPgo8Y2lyY2xlIHN0eWxlPSJmaWxsOiNGRkRBNDQ7IiBjeD0iMTk2LjY0MSIgY3k9IjI1NiIgcj0iNjQiLz4KPHBhdGggc3R5bGU9ImZpbGw6I0Q4MDAyNzsiIGQ9Ik0xNjAuNjM4LDIyNHY0MC4wMDFjMCwxOS44ODIsMTYuMTE4LDM2LDM2LDM2czM2LTE2LjExOCwzNi0zNlYyMjRIMTYwLjYzOHoiLz4KPHBhdGggc3R5bGU9ImZpbGw6I0YwRjBGMDsiIGQ9Ik0xOTYuNjM4LDI3NmMtNi42MTcsMC0xMi01LjM4My0xMi0xMnYtMTZoMjQuMDAxdjE2QzIwOC42MzgsMjcwLjYxNiwyMDMuMjU0LDI3NiwxOTYuNjM4LDI3NnoiLz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPGc+CjwvZz4KPC9zdmc+Cg==">
  </a> -->
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_zh-CN.md">
    <img height="20px" src="https://img.shields.io/badge/CN-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxMjAwIDgwMCIgeG1sbnM6eGxpbms9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkveGxpbmsiPg0KPHBhdGggZmlsbD0iI2RlMjkxMCIgZD0ibTAsMGgxMjAwdjgwMGgtMTIwMHoiLz4NCjxwYXRoIGZpbGw9IiNmZmRlMDAiIGQ9Im0tMTYuNTc5Niw5OS42MDA3bDIuMzY4Ni04LjEwMzItNi45NTMtNC43ODgzIDguNDM4Ni0uMjUxNCAyLjQwNTMtOC4wOTI0IDIuODQ2Nyw3Ljk0NzkgOC40Mzk2LS4yMTMxLTYuNjc5Miw1LjE2MzQgMi44MTA2LDcuOTYwNy02Ljk3NDctNC43NTY3LTYuNzAyNSw1LjEzMzF6IiB0cmFuc2Zvcm09Im1hdHJpeCg5LjkzMzUyIC4yNzc0NyAtLjI3NzQ3IDkuOTMzNTIgMzI0LjI5MjUgLTY5NS4yNDE1KSIvPg0KPHBhdGggZmlsbD0iI2ZmZGUwMCIgaWQ9InN0YXIiIGQ9Im0zNjUuODU1MiwzMzIuNjg5NWwyOC4zMDY4LDExLjM3NTcgMTkuNjcyMi0yMy4zMTcxLTIuMDcxNiwzMC40MzY3IDI4LjI1NDksMTEuNTA0LTI5LjU4NzIsNy40MzUyLTIuMjA5NywzMC40MjY5LTE2LjIxNDItMjUuODQxNS0yOS42MjA2LDcuMzAwOSAxOS41NjYyLTIzLjQwNjEtMTYuMDk2OC0yNS45MTQ4eiIvPg0KPGcgZmlsbD0iI2ZmZGUwMCI+DQo8cGF0aCBkPSJtNTE5LjA3NzksMTc5LjMxMjlsLTMwLjA1MzQtNS4yNDE4LTE0LjM5NDUsMjYuODk3Ni00LjMwMTctMzAuMjAyMy0zMC4wMjkzLTUuMzc4MSAyNy4zOTQ4LTEzLjQyNDItNC4xNjQ3LTMwLjIyMTUgMjEuMjMyNiwyMS45MDU3IDI3LjQ1NTQtMTMuMjk5OC0xNC4yNzIzLDI2Ljk2MjcgMjEuMTMzMSwyMi4wMDE3eiIvPg0KPHBhdGggZD0ibTQ1NS4yNTkyLDMxNS45Nzk1bDkuMzczNC0yOS4wMzE0LTI0LjYzMjUtMTcuOTk3OCAzMC41MDctLjA1NjYgOS41MDUtMjguOTg4NiA5LjQ4MSwyOC45OTY0IDMwLjUwNywuMDgxOC0yNC42NDc0LDE3Ljk3NzQgOS4zNDkzLDI5LjAzOTItMjQuNzE0LTE3Ljg4NTgtMjQuNzI4OCwxNy44NjUzeiIvPg0KPC9nPg0KPHVzZSB4bGluazpocmVmPSIjc3RhciIgdHJhbnNmb3JtPSJtYXRyaXgoLjk5ODYzIC4wNTIzNCAtLjA1MjM0IC45OTg2MyAxOS40MDAwNSAtMzAwLjUzNjgxKSIvPg0KPC9zdmc+DQo=">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_zh-TW.md">
    <img height="20px" src="https://img.shields.io/badge/TW-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4NCjwhRE9DVFlQRSBzdmc+DQo8c3ZnIHdpZHRoPSI5MDAiIGhlaWdodD0iNjAwIiB2aWV3Qm94PSItNjAgLTQwIDI0MCAxNjAiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgeG1sbnM6eGxpbms9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkveGxpbmsiPg0KICAgPHJlY3QgeD0iLTYwIiB5PSItNDAiIHdpZHRoPSIxMDAlIiBoZWlnaHQ9IjEwMCUiIGZpbGw9IiNmZTAwMDAiLz4NCiAgIDxyZWN0IHg9Ii02MCIgeT0iLTQwIiB3aWR0aD0iNTAlIiBoZWlnaHQ9IjUwJSIgZmlsbD0iIzAwMDA5NSIvPg0KICAgPHBhdGggaWQ9ImZvdXJfcmF5cyIgZD0iTSA4LDAgTCAwLDMwIEwgLTgsMCBMIDAsLTMwIE0gMCw4IEwgMzAsMCBMIDAsLTggTCAtMzAsMCIgZmlsbD0iI2ZmZiIvPg0KICAgPHVzZSB4bGluazpocmVmPSIjZm91cl9yYXlzIiB0cmFuc2Zvcm09InJvdGF0ZSgzMCkiLz4NCiAgIDx1c2UgeGxpbms6aHJlZj0iI2ZvdXJfcmF5cyIgdHJhbnNmb3JtPSJyb3RhdGUoNjApIi8+DQogICA8Y2lyY2xlIHI9IjE3IiBmaWxsPSIjMDAwMDk1Ii8+DQogICA8Y2lyY2xlIHI9IjE1IiBmaWxsPSIjZmZmIi8+DQo8L3N2Zz4=">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_de.md">
    <img height="20px" src="https://img.shields.io/badge/DE-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTAwMCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiBoZWlnaHQ9IjYwMCIgdmlld0JveD0iMCAwIDUgMyI+DQo8cGF0aCBkPSJtMCwwaDV2M2gtNXoiLz4NCjxwYXRoIGZpbGw9IiNkMDAiIGQ9Im0wLDFoNXYyaC01eiIvPg0KPHBhdGggZmlsbD0iI2ZmY2UwMCIgZD0ibTAsMmg1djFoLTV6Ii8+DQo8L3N2Zz4NCg==">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_nl.md">
    <img height="20px" src="https://img.shields.io/badge/NL-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iOTAwIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIGhlaWdodD0iNjAwIiB2aWV3Qm94PSIwIDAgOSA2Ij4NCjxwYXRoIGZpbGw9IiMyMTQ2OGIiIGQ9Im0wLDBoOXY2aC05eiIvPg0KPHBhdGggZmlsbD0iI2ZmZiIgZD0ibTAsMGg5djRoLTl6Ii8+DQo8cGF0aCBmaWxsPSIjYWUxYzI4IiBkPSJtMCwwaDl2MmgtOXoiLz4NCjwvc3ZnPg0K">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://img.shields.io/badge/KO-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iOTAwIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIGhlaWdodD0iNjAwIiB2aWV3Qm94PSItMzYgLTI0IDcyIDQ4IiB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayI+DQo8cGF0aCBmaWxsPSIjZmZmIiBkPSJtLTM2LTI0aDcydjQ4aC03MnoiLz4NCjxnIHRyYW5zZm9ybT0ibWF0cml4KC41NTQ3IC0uODMyMDUgLjgzMjA1IC41NTQ3IDAgMCkiPg0KPGcgaWQ9ImIyIj4NCjxwYXRoIHN0cm9rZT0iIzAwMCIgaWQ9ImIiIHN0cm9rZS13aWR0aD0iMiIgZD0iTS02LTI1SDZNLTYtMjJINk0tNi0xOUg2Ii8+DQo8dXNlIHk9IjQ0IiB4bGluazpocmVmPSIjYiIvPg0KPC9nPg0KPHBhdGggc3Ryb2tlPSIjZmZmIiBkPSJtMCwxN3YxMCIvPg0KPGNpcmNsZSBmaWxsPSIjYzYwYzMwIiByPSIxMiIvPg0KPHBhdGggZmlsbD0iIzAwMzQ3OCIgZD0iTTAtMTJBNiw2IDAgMCAwIDAsMEE2LDYgMCAwIDEgMCwxMkExMiwxMiAwIDAsMSAwLTEyWiIvPg0KPC9nPg0KPGcgdHJhbnNmb3JtPSJtYXRyaXgoLS41NTQ3IC0uODMyMDUgLjgzMjA1IC0uNTU0NyAwIDApIj4NCjx1c2UgeGxpbms6aHJlZj0iI2IyIi8+DQo8cGF0aCBzdHJva2U9IiNmZmYiIGQ9Im0wLTIzLjV2M20wLDM3LjV2My41bTAsM3YzIi8+DQo8L2c+DQo8L3N2Zz4NCg==">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://img.shields.io/badge/FR-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA5MDAgNjAwIj4NCjxwYXRoIGZpbGw9IiNlZDI5MzkiIGQ9Im0wLDBoOTAwdjYwMGgtOTAweiIvPg0KPHBhdGggZmlsbD0iI2ZmZiIgZD0ibTAsMGg2MDB2NjAwaC02MDB6Ii8+DQo8cGF0aCBmaWxsPSIjMDAyMzk1IiBkPSJtMCwwaDMwMHY2MDBoLTMwMHoiLz4NCjwvc3ZnPg0K">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_tr.md">
    <img height="20px" src="https://img.shields.io/badge/TR-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTIwMCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiBoZWlnaHQ9IjgwMCIgdmlld0JveD0iMCAwIDM2MCAyNDAiIHhtbG5zOnhsaW5rPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rIj4NCjxwYXRoIGZpbGw9IiNlMzBhMTciIGQ9Im0wLDBoMzYwdjI0MGgtMzYweiIvPg0KPGNpcmNsZSBmaWxsPSIjZmZmIiBjeD0iMTIwIiBjeT0iMTIwIiByPSI2MCIvPg0KPGNpcmNsZSBmaWxsPSIjZTMwYTE3IiBjeD0iMTM1IiBjeT0iMTIwIiByPSI0OCIvPg0KPGcgZmlsbD0iI2ZmZiIgdHJhbnNmb3JtPSJtYXRyaXgoMCAtMzAgMzAgMCAyMDAuNyAxMjApIj4NCjxnIGlkPSJnMiI+DQo8cGF0aCBpZD0iZzEiIGQ9Im0wLDAgMCwxIC41LDB6IiB0cmFuc2Zvcm09Im1hdHJpeCguOTUxMDYgLjMwOTAyIC0uMzA5MDIgLjk1MTA2IDAgLTEpIi8+DQo8dXNlIHhsaW5rOmhyZWY9IiNnMSIgdHJhbnNmb3JtPSJzY2FsZSgtMSAxKSIvPg0KPC9nPg0KPHVzZSB4bGluazpocmVmPSIjZzIiIHRyYW5zZm9ybT0icm90YXRlKDcyKSIvPg0KPHVzZSB4bGluazpocmVmPSIjZzIiIHRyYW5zZm9ybT0ibWF0cml4KC4zMDkwMiAtLjk1MTA2IC45NTEwNiAuMzA5MDIgMCAwKSIvPg0KPHVzZSB4bGluazpocmVmPSIjZzIiIHRyYW5zZm9ybT0icm90YXRlKDE0NCkiLz4NCjx1c2UgeGxpbms6aHJlZj0iI2cyIiB0cmFuc2Zvcm09Im1hdHJpeCgtLjgwOTAyIC0uNTg3NzkgLjU4Nzc5IC0uODA5MDIgMCAwKSIvPg0KPC9nPg0KPC9zdmc+DQo=">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_id.md">
    <img height="20px" src="https://img.shields.io/badge/ID-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA2MDAgNDAwIj4NCjxwYXRoIGZpbGw9IiNjZTExMjYiIGQ9Im0wLDBoNjAwdjIwMGgtNjAweiIvPg0KPHBhdGggZmlsbD0iI2ZmZiIgZD0ibTAsMjAwaDYwMHYyMDBoLTYwMHoiLz4NCjwvc3ZnPg0K">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_he.md">
    <img height="20px" src="https://img.shields.io/badge/HE-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNjYwIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIGhlaWdodD0iNDgwIiB2aWV3Qm94PSIwIDAgMjIwIDE2MCIgeG1sbnM6eGxpbms9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkveGxpbmsiPg0KPGRlZnM+DQo8cGF0aCBzdHJva2U9IiMwMDM4YjgiIGZpbGwtb3BhY2l0eT0iMCIgaWQ9InRyaWFuZ2xlIiBzdHJva2Utd2lkdGg9IjUuNSIgZD0ibTAtMjkuMTQxLTI1LjIzNjksNDMuNzExNSA1MC40NzM4LTB6Ii8+DQo8L2RlZnM+DQo8cGF0aCBmaWxsPSIjZmZmIiBkPSJtMCwwaDIyMHYxNjBoLTIyMHoiLz4NCjxnIGZpbGw9IiMwMDM4YjgiPg0KPHBhdGggZD0ibTAsMTVoMjIwdjI1aC0yMjB6Ii8+DQo8cGF0aCBkPSJtMCwxMjBoMjIwdjI1aC0yMjB6Ii8+DQo8L2c+DQo8dXNlIHhsaW5rOmhyZWY9IiN0cmlhbmdsZSIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMTEwIDgwKSIvPg0KPHVzZSB4bGluazpocmVmPSIjdHJpYW5nbGUiIHRyYW5zZm9ybT0ibWF0cml4KC0xIDAgLTAgLTEgMTEwIDgwKSIvPg0KPC9zdmc+DQo=">
  </a>
   <a href="https://github.com/gofiber/fiber/blob/master/.github/README_sa.md">
     <img height="20px" src="https://img.shields.io/badge/SA-flag.svg?color=555555&style=flat&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTIwMCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiBoZWlnaHQ9IjYwMCIgdmlld0JveD0iMCAwIDEyIDYiIHhtbG5zOnhsaW5rPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rIj4NCjxwYXRoIGZpbGw9IiNjZTExMjYiIGQ9Im0wLDBoM3Y2aC0zeiIvPg0KPHBhdGggZmlsbD0iIzAwOWEwMCIgZD0ibTMsMGg5djJoLTl6Ii8+DQo8cGF0aCBmaWxsPSIjZmZmIiBkPSJtMywyaDl2MmgtOXoiLz4NCjxwYXRoIGQ9Im0zLDRoOXYyaC05eiIvPg0KPC9zdmc+DQo=">
   </a>
  <br>
  <a href="https://pkg.go.dev/github.com/gofiber/fiber/v2?tab=doc">
    <img src="https://img.shields.io/badge/%F0%9F%93%9A%20godoc-pkg-00ACD7.svg?color=00ACD7&style=flat">
  </a>
  <a href="https://goreportcard.com/report/github.com/gofiber/fiber">
    <img src="https://img.shields.io/badge/%F0%9F%93%9D%20goreport-A%2B-75C46B">
  </a>
  <a href="https://gocover.io/github.com/gofiber/fiber">
    <img src="https://img.shields.io/badge/%F0%9F%94%8E%20gocover-97.8%25-75C46B.svg?style=flat">
  </a>
  <a href="https://github.com/gofiber/fiber/actions?query=workflow%3ASecurity>
    <img src="https://img.shields.io/github/workflow/status/gofiber/fiber/Security?label=%F0%9F%94%91%20gosec&style=flat&color=75C46B">
  </a>
  <a href="https://github.com/gofiber/fiber/actions?query=workflow%3ATest">
    <img src="https://img.shields.io/github/workflow/status/gofiber/fiber/Test?label=%F0%9F%A7%AA%20tests&style=flat&color=75C46B">
  </a>
    <a href="https://docs.gofiber.io">
    <img src="https://img.shields.io/badge/%F0%9F%92%A1%20fiber-docs-00ACD7.svg?style=flat">
  </a>
  <a href="https://gofiber.io/discord">
    <img src="https://img.shields.io/discord/704680098577514527?style=flat&label=%F0%9F%92%AC%20discord&color=00ACD7">
  </a>
</p>
<p align="center">
<b>Fiber</b> é um <b>framework web</b> inspirado no <a href="https://github.com/expressjs/express">Express</a>, construído sobre o <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, o motor HTTP <b>mais rápido</b> do <a href="https://golang.org/doc/">Go</a>. Projetado para <b>facilitar</b> e <b>acelerar</b> o desenvolvimento, com <b>zero de alocação de memória</b> e <b>desempenho</b> em mente.
</p>

## ⚡️ Início rápido

```go
package main

import "github.com/gofiber/fiber/v2"

func main() {
    app := fiber.New()

    app.Get("/", func(c *fiber.Ctx) error {
        return c.SendString("Hello, World 👋!")
    })

    app.Listen(":3000")
}
```

## 🤖 Benchmarks

Esses testes são realizados pelo [TechEmpower](https://www.techempower.com/benchmarks/#section=data-r19&hw=ph&test=plaintext) e [Go Web](https://github.com/smallnest/go-web-framework-benchmark). Se você quiser ver todos os resultados, visite nosso [Wiki](https://docs.gofiber.io/benchmarks) .

<p float="left" align="middle">
  <img src="https://raw.githubusercontent.com/gofiber/docs/master/.gitbook/assets/benchmark-pipeline.png" width="49%">
  <img src="https://raw.githubusercontent.com/gofiber/docs/master/.gitbook/assets/benchmark_alloc.png" width="49%">
</p>

## ⚙️ Instalação

Make sure you have Go installed ([download](https://golang.org/dl/)). Version `1.14` or higher is required. 

Initialize your project by creating a folder and then running `go mod init github.com/your/repo` ([learn more](https://blog.golang.org/using-go-modules)) inside the folder. Then install Fiber with the [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) command:

```bash
go get -u github.com/gofiber/fiber/v2
```

## 🎯 Recursos

-   [Roteamento](https://docs.gofiber.io/routing) robusto
-   Servir [arquivos estáticos](https://docs.gofiber.io/api/app#static)
-   [Desempenho](https://docs.gofiber.io/benchmarks) extremo
-   [Baixo consumo de memória](https://docs.gofiber.io/benchmarks)
-   [API de rotas](https://docs.gofiber.io/api/ctx)
-   Suporte à Middleware e [Next](https://docs.gofiber.io/api/ctx#next)
-   Programação [rápida](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) de aplicações de servidor
-   [Templates](https://github.com/gofiber/template)
-   [Suporte à WebSockets](https://github.com/gofiber/websocket)
-   [Limitador de requisições](https://docs.gofiber.io/middleware#limiter)
-   Disponível em [15 línguas](https://docs.gofiber.io/)
-   E muito mais, [explore o Fiber](https://docs.gofiber.io/)

## 💡 Filosofia

Os novos gophers que mudaram do [Node.js](https://nodejs.org/en/about/) para o [Go](https://golang.org/doc/) estão tendo que lidar com uma curva de aprendizado antes que possam começar a criar seus aplicativos web ou microsserviços. O Fiber, como um **framework web**, foi criado com a ideia de ser **minimalista** e seguindo a **filosofia UNIX**, para que novos gophers possam, rapidamente, entrar no mundo do Go com uma recepção calorosa e confiável.

O Fiber é **inspirado** no Express, o framework web mais popular da Internet. Combinamos a **facilidade** do Express e com o **desempenho bruto** do Go. Se você já implementou um aplicativo web com Node.js ( _usando Express.js ou similar_ ), então muitos métodos e princípios parecerão **muito familiares** para você.

## 👀 Exemplos

Listados abaixo estão alguns exemplos comuns. Se você quiser ver mais exemplos de código,
visite nosso [repositório de receitas](https://github.com/gofiber/recipes) ou
a [documentação da API](https://docs.gofiber.io).

#### 📖 [**Roteamento básico**](https://docs.gofiber.io/#basic-routing)

```go
func main() {
    app := fiber.New()

    // GET /john
    app.Get("/:name", func(c *fiber.Ctx) error {
        msg := fmt.Sprintf("Hello, %s 👋!", c.Params("name"))
        return c.SendString(msg) // => Hello john 👋!
    })

    // GET /john/75
    app.Get("/:name/:age", func(c *fiber.Ctx) error {
        msg := fmt.Sprintf("👴 %s is %s years old", c.Params("name"), c.Params("age"))
        return c.SendString(msg) // => 👴 john is 75 years old
    })

    // GET /dictionary.txt
    app.Get("/:file.:ext", func(c *fiber.Ctx) error {
        msg := fmt.Sprintf("📃 %s.%s", c.Params("file"), c.Params("ext"))
        return c.SendString(msg) // => 📃 dictionary.txt
    })

    // GET /flights/LAX-SFO
    app.Get("/flights/:from-:to", func(c *fiber.Ctx) error {
        msg := fmt.Sprintf("💸 From: %s, To: %s", c.Params("from"), c.Params("to"))
        return c.SendString(msg) // => 💸 From: LAX, To: SFO
    })

    // GET /api/register
    app.Get("/api/*", func(c *fiber.Ctx) error {
        msg := fmt.Sprintf("✋ %s", c.Params("*"))
        return c.SendString(msg) // => ✋ register
    })

    log.Fatal(app.Listen(":3000"))
}

```

#### 📖 [**Servindo arquivos estáticos**](https://docs.gofiber.io/api/app#static)

```go
func main() {
    app := fiber.New()

    app.Static("/", "./public")
    // => http://localhost:3000/js/script.js
    // => http://localhost:3000/css/style.css

    app.Static("/prefix", "./public")
    // => http://localhost:3000/prefix/js/script.js
    // => http://localhost:3000/prefix/css/style.css

    app.Static("*", "./public/index.html")
    // => http://localhost:3000/any/path/shows/index/html

    log.Fatal(app.Listen(":3000"))
}

```

#### 📖 [**Middleware & Next**](https://docs.gofiber.io/api/ctx#next)

```go
func main() {
	app := fiber.New()

	// Match any route
	app.Use(func(c *fiber.Ctx) error {
		fmt.Println("🥇 First handler")
		return c.Next()
	})

	// Match all routes starting with /api
	app.Use("/api", func(c *fiber.Ctx) error {
		fmt.Println("🥈 Second handler")
		return c.Next()
	})

	// GET /api/register
	app.Get("/api/list", func(c *fiber.Ctx) error {
		fmt.Println("🥉 Last handler")
		return c.SendString("Hello, World 👋!")
	})

	log.Fatal(app.Listen(":3000"))
}

```

<details>
  <summary>📚 Mostrar mais exemplos</summary>

### Engines de visualização

📖 [Config](https://docs.gofiber.io/fiber#config)
📖 [Engines](https://github.com/gofiber/template)
📖 [Render](https://docs.gofiber.io/context#render)

O Fiber usa por padrão o [html/template](https://golang.org/pkg/html/template/) quando nenhuma engine é selecionada.

Se você quiser uma execução parcial ou usar uma engine diferente como [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) ou [pug](https://github.com/Joker/jade) etc.. Dê uma olhada no package [Template](https://github.com/gofiber/template) que suporta multiplas engines de visualização.

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/template/pug"
)

func main() {
    // You can setup Views engine before initiation app:
    app := fiber.New(fiber.Config{
        Views: pug.New("./views", ".pug"),
    })

    // And now, you can call template `./views/home.pug` like this:
    app.Get("/", func(c *fiber.Ctx) error {
        return c.Render("home", fiber.Map{
            "title": "Homepage",
            "year":  1999,
        })
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Agrupamento de rotas

📖 [Group](https://docs.gofiber.io/application#group)

```go
func middleware(c *fiber.Ctx) error {
    fmt.Println("Don't mind me!")
    return c.Next()
}

func handler(c *fiber.Ctx) error {
    return c.SendString(c.Path())
}

func main() {
    app := fiber.New()

    // Root API route
    api := app.Group("/api", middleware) // /api

    // API v1 routes
    v1 := api.Group("/v1", middleware) // /api/v1
    v1.Get("/list", handler)           // /api/v1/list
    v1.Get("/user", handler)           // /api/v1/user

    // API v2 routes
    v2 := api.Group("/v2", middleware) // /api/v2
    v2.Get("/list", handler)           // /api/v2/list
    v2.Get("/user", handler)           // /api/v2/user

    // ...
}

```

### Middleware Logger

📖 [Logger](https://docs.gofiber.io/middleware/logger)

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
    app := fiber.New()

    app.Use(logger.New())

    // ...

    log.Fatal(app.Listen(":3000"))
}
```

### Cross-Origin Resource Sharing (CORS)

📖 [CORS](https://docs.gofiber.io/middleware/cors)

```go
import (
    "log"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
    app := fiber.New()

    app.Use(cors.New())

    // ...

    log.Fatal(app.Listen(":3000"))
}
```

Verifique o CORS passando qualquer domínio no header `Origin`:

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```

### Resposta 404 customizada

📖 [HTTP Methods](https://docs.gofiber.io/application#http-methods)

```go
func main() {
    app := fiber.New()

    app.Static("/", "./public")

    app.Get("/demo", func(c *fiber.Ctx) error {
        return c.SendString("This is a demo!")
    })

    app.Post("/register", func(c *fiber.Ctx) error {
        return c.SendString("Welcome!")
    })

    // Last middleware to match anything
    app.Use(func(c *fiber.Ctx) error {
        return c.SendStatus(404)
        // => 404 "Not Found"
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Resposta JSON

📖 [JSON](https://docs.gofiber.io/ctx#json)

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func main() {
    app := fiber.New()

    app.Get("/user", func(c *fiber.Ctx) error {
        return c.JSON(&User{"John", 20})
        // => {"name":"John", "age":20}
    })

    app.Get("/json", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{
            "success": true,
            "message": "Hi John!",
        })
        // => {"success":true, "message":"Hi John!"}
    })

    log.Fatal(app.Listen(":3000"))
}
```

### WebSocket

📖 [Websocket](https://github.com/gofiber/websocket)

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/websocket"
)

func main() {
  app := fiber.New()

  app.Get("/ws", websocket.New(func(c *websocket.Conn) {
    for {
      mt, msg, err := c.ReadMessage()
      if err != nil {
        log.Println("read:", err)
        break
      }
      log.Printf("recv: %s", msg)
      err = c.WriteMessage(mt, msg)
      if err != nil {
        log.Println("write:", err)
        break
      }
    }
  }))

  log.Fatal(app.Listen(":3000"))
  // ws://localhost:3000/ws
}
```

### Middleware Recover

📖 [Recover](https://docs.gofiber.io/middleware/recover)

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
    app := fiber.New()

    app.Use(recover.New())

    app.Get("/", func(c *fiber.Ctx) error {
        panic("normally this would crash your app")
    })

    log.Fatal(app.Listen(":3000"))
}
```

</details>

## 🧬 Internal Middleware

Here is a list of middleware that are included within the Fiber framework.

| Middleware                                                                       | Description                                                                                                                                                           |
| :------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [basicauth](https://github.com/gofiber/fiber/tree/master/middleware/basicauth)   | Basic auth middleware provides an HTTP basic authentication. It calls the next handler for valid credentials and 401 Unauthorized for missing or invalid credentials. |
| [compress](https://github.com/gofiber/fiber/tree/master/middleware/compress)     | Compression middleware for Fiber, it supports `deflate`, `gzip` and `brotli` by default.                                                                              |
| [cache](https://github.com/gofiber/fiber/tree/master/middleware/cache)           | Intercept and cache responses                                                                                                                                         |
| [cors](https://github.com/gofiber/fiber/tree/master/middleware/cors)             | Enable cross-origin resource sharing \(CORS\) with various options.                                                                                                   |
| [csrf](https://github.com/gofiber/fiber/tree/master/middleware/csrf)             | Protect from CSRF exploits.                                                                                                                                           |
| [filesystem](https://github.com/gofiber/fiber/tree/master/middleware/filesystem) | FileSystem middleware for Fiber, special thanks and credits to Alireza Salary                                                                                         |
| [favicon](https://github.com/gofiber/fiber/tree/master/middleware/favicon)       | Ignore favicon from logs or serve from memory if a file path is provided.                                                                                             |
| [limiter](https://github.com/gofiber/fiber/tree/master/middleware/limiter)       | Rate-limiting middleware for Fiber. Use to limit repeated requests to public APIs and/or endpoints such as password reset.                                            |
| [logger](https://github.com/gofiber/fiber/tree/master/middleware/logger)         | HTTP request/response logger.                                                                                                                                         |
| [pprof](https://github.com/gofiber/fiber/tree/master/middleware/pprof)           | Special thanks to Matthew Lee \(@mthli\)                                                                                                                              |
| [proxy](https://github.com/gofiber/fiber/tree/master/middleware/proxy)           | Allows you to proxy requests to a multiple servers                                                                                                                    |
| [requestid](https://github.com/gofiber/fiber/tree/master/middleware/requestid)   | Adds a requestid to every request.                                                                                                                                    |
| [recover](https://github.com/gofiber/fiber/tree/master/middleware/recover)       | Recover middleware recovers from panics anywhere in the stack chain and handles the control to the centralized[ ErrorHandler](error-handling.md).                     |
| [timeout](https://github.com/gofiber/fiber/tree/master/middleware/timeout)       | Adds a max time for a request and forwards to ErrorHandler if it is exceeded.                                                                                         |

## 🧬 External Middleware

List of externally hosted middleware modules and maintained by the [Fiber team](https://github.com/orgs/gofiber/people).

| Middleware                                        | Description                                                                                                                                                         |
| :------------------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [adaptor](https://github.com/gofiber/adaptor)     | Converter for net/http handlers to/from Fiber request handlers, special thanks to @arsmn!                                                                           |
| [helmet](https://github.com/gofiber/helmet)       | Helps secure your apps by setting various HTTP headers.                                                                                                             |
| [jwt](https://github.com/gofiber/jwt)             | JWT returns a JSON Web Token \(JWT\) auth middleware.                                                                                                               |
| [keyauth](https://github.com/gofiber/keyauth)     | Key auth middleware provides a key based authentication.                                                                                                            |
| [rewrite](https://github.com/gofiber/rewrite)     | Rewrite middleware rewrites the URL path based on provided rules. It can be helpful for backward compatibility or just creating cleaner and more descriptive links. |
| [session](https://github.com/gofiber/session)     | This session middleware is build on top of fasthttp/session by @savsgio MIT. Special thanks to @thomasvvugt for helping with this middleware.                       |
| [template](https://github.com/gofiber/template)   | This package contains 8 template engines that can be used with Fiber `v1.10.x` Go version 1.13 or higher is required.                                               |
| [websocket](https://github.com/gofiber/websocket) | Based on Fasthttp WebSocket for Fiber with Locals support!                                                                                                          |

## 🌱 Middlewares Third Party

Esta é uma lista de middlewares criados pela comunidade do Fiber, se quiser ter o seu middleware aqui, é só abrir um PR!

-   [arsmn/fiber-casbin](https://github.com/arsmn/fiber-casbin)
-   [arsmn/fiber-introspect](https://github.com/arsmn/fiber-introspect)
-   [arsmn/fiber-swagger](https://github.com/arsmn/fiber-swagger)
-   [arsmn/gqlgen](https://github.com/arsmn/gqlgen)
-   [codemicro/fiber-cache](https://github.com/codemicro/fiber-cache)
-   [sujit-baniya/fiber-boilerplate](https://github.com/sujit-baniya/fiber-boilerplate)
-   [juandiii/go-jwk-security](https://github.com/juandiii/go-jwk-security)
-   [kiyonlin/fiber_limiter](https://github.com/kiyonlin/fiber_limiter)
-   [shareed2k/fiber_limiter](https://github.com/shareed2k/fiber_limiter)
-   [shareed2k/fiber_tracing](https://github.com/shareed2k/fiber_tracing)
-   [thomasvvugt/fiber-boilerplate](https://github.com/thomasvvugt/fiber-boilerplate)
-   [ansrivas/fiberprometheus](https://github.com/ansrivas/fiberprometheus)
-   [LdDl/fiber-long-poll](https://github.com/LdDl/fiber-long-poll)
-   [K0enM/fiber_vhost](https://github.com/K0enM/fiber_vhost)
-   [theArtechnology/fiber-inertia](https://github.com/theArtechnology/fiber-inertia)

## 👍 Contribuindo

Se você quer **agradecer** e/ou apoiar o desenvolvimento ativo do `Fiber`:

1. Deixe uma [estrela no GitHub](https://github.com/gofiber/fiber/stargazers) do projeto.
2. Tweet sobre o projeto [no seu Twitter](https://twitter.com/intent/tweet?text=Fiber%20is%20an%20Express%20inspired%20%23web%20%23framework%20built%20on%20top%20of%20Fasthttp%2C%20the%20fastest%20HTTP%20engine%20for%20%23Go.%20Designed%20to%20ease%20things%20up%20for%20%23fast%20development%20with%20zero%20memory%20allocation%20and%20%23performance%20in%20mind%20%F0%9F%9A%80%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Escreva um review ou tutorial no [Medium](https://medium.com/), [Dev.to](https://dev.to/) ou blog pessoal.
4. Apoie o projeto pagando uma [xícara de café](https://buymeacoff.ee/fenny).

## ☕ Apoiadores

Fiber é um projeto open source que usa de doações para pagar seus custos (domínio, GitBook, Netlify e hospedagem serverless). Se você quiser apoiar o projeto, você pode ☕ [**pagar um café**](https://buymeacoff.ee/fenny).

|                                                            | User                                             | Donation |
| :--------------------------------------------------------- | :----------------------------------------------- | :------- |
| ![](https://avatars.githubusercontent.com/u/204341?s=25)   | [@destari](https://github.com/destari)           | ☕ x 10  |
| ![](https://avatars.githubusercontent.com/u/63164982?s=25) | [@dembygenesis](https://github.com/dembygenesis) | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/56607882?s=25) | [@thomasvvugt](https://github.com/thomasvvugt)   | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/27820675?s=25) | [@hendratommy](https://github.com/hendratommy)   | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/1094221?s=25)  | [@ekaputra07](https://github.com/ekaputra07)     | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/194590?s=25)   | [@jorgefuertes](https://github.com/jorgefuertes) | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/186637?s=25)   | [@candidosales](https://github.com/candidosales) | ☕ x 5   |
| ![](https://avatars.githubusercontent.com/u/29659953?s=25) | [@l0nax](https://github.com/l0nax)               | ☕ x 3   |
| ![](https://avatars.githubusercontent.com/u/59947262?s=25) | [@ankush](https://github.com/ankush)             | ☕ x 3   |
| ![](https://avatars.githubusercontent.com/u/635852?s=25)   | [@bihe](https://github.com/bihe)                 | ☕ x 3   |
| ![](https://avatars.githubusercontent.com/u/307334?s=25)   | [@justdave](https://github.com/justdave)         | ☕ x 3   |
| ![](https://avatars.githubusercontent.com/u/11155743?s=25) | [@koddr](https://github.com/koddr)               | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/29042462?s=25) | [@lapolinar](https://github.com/lapolinar)       | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/2978730?s=25)  | [@diegowifi](https://github.com/diegowifi)       | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/44171355?s=25) | [@ssimk0](https://github.com/ssimk0)             | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/5638101?s=25)  | [@raymayemir](https://github.com/raymayemir)     | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/619996?s=25)   | [@melkorm](https://github.com/melkorm)           | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/31022056?s=25) | [@marvinjwendt](https://github.com/thomasvvugt)  | ☕ x 1   |
| ![](https://avatars.githubusercontent.com/u/31921460?s=25) | [@toishy](https://github.com/toishy)             | ☕ x 1   |

## ‎‍💻 Contribuidores de código

<img src="https://opencollective.com/fiber/contributors.svg?width=890&button=false" alt="Code Contributors" style="max-width:100%;">

## ⭐️ Stargazers

<img src="https://starchart.cc/gofiber/fiber.svg" alt="Stargazers over time" style="max-width: 100%">

## ⚠️ Licença

Todos os direitos reservados (c) 2019-presente [Fenny](https://github.com/fenny) e [Contribuidores](https://github.com/gofiber/fiber/graphs/contributors).
`Fiber` é software livre e aberto sob a [licença MIT](https://github.com/gofiber/fiber/blob/master/LICENSE).
O logo oficial foi criado por [Vic Shóstak](https://github.com/koddr) e distribuído sob a licença [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) (CC BY-SA 4.0 International).

**Licença das bibliotecas de terceiros**

-   [schema](https://github.com/gorilla/schema/blob/master/LICENSE)
-   [isatty](https://github.com/mattn/go-isatty/blob/master/LICENSE)
-   [fasthttp](https://github.com/valyala/fasthttp/blob/master/LICENSE)
-   [encoding](https://github.com/segmentio/encoding/blob/master/LICENSE)
-   [colorable](https://github.com/mattn/go-colorable/blob/master/LICENSE)
-   [fasttemplate](https://github.com/valyala/fasttemplate/blob/master/LICENSE)
-   [bytebufferpool](https://github.com/valyala/bytebufferpool/blob/master/LICENSE)
-   [gopsutil](https://github.com/shirou/gopsutil/blob/master/LICENSE)
-   [go-ole](https://github.com/go-ole/go-ole)
-   [wmi](https://github.com/StackExchange/wmi)
