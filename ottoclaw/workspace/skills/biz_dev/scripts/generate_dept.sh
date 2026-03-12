#!/bin/bash
# generate_dept.sh [DeptName] [OrgName]
DEPT=$1
ORG=${2:-"Siam-Synapse Corp"}

if [ -z "$DEPT" ]; then
  echo "Error: Department name required"
  exit 1
fi

echo "🏗️  Scaffolding Department: $DEPT (Org: $ORG)"

# Create a template directory for the new department
DEPT_DIR="/tmp/siam_templates/$DEPT"
mkdir -p "$DEPT_DIR"

echo "$DEPT" > "$DEPT_DIR/DEPARTMENT"
echo "$ORG" > "$DEPT_DIR/ORG_ID"
echo "Role: staff" > "$DEPT_DIR/ROLE"

echo "✅ Department template created in $DEPT_DIR"
echo "You can now use this to spawn new agents for this department."
