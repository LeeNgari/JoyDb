import re

with open('internal/domain/data/row.go', 'r') as f:
    content = f.read()

# Remove UnmarshalJSON
content = re.sub(r'// UnmarshalJSON implements json\.Unmarshaler interface\n// This allows Row to be unmarshaled from JSON as a map\nfunc \(r \*Row\) UnmarshalJSON\(data \[\]byte\) error \{\n.*?return nil\n\}\n\n', '', content, flags=re.DOTALL)

# Remove MarshalJSON
content = re.sub(r'// MarshalJSON implements json\.Marshaler interface\n// This allows Row to be marshaled to JSON as a map\nfunc \(r Row\) MarshalJSON\(\) \(\[\]byte, error\) \{\n.*?return json\.Marshal\(r\.Data\)\n\}\n\n', '', content, flags=re.DOTALL)

# Remove "encoding/json"
content = content.replace('\t"encoding/json"\n', '')

with open('internal/domain/data/row.go', 'w') as f:
    f.write(content)
