#!/bin/sh
# Entrypoint for picoclaw Docker container.
# Syncs built-in skills to the workspace volume on first run,
# so volume mounts don't hide embedded skills.

WORKSPACE="$HOME/.picoclaw/workspace"
SKILLS_SRC="$HOME/.picoclaw/_builtin_skills"
SKILLS_DST="$WORKSPACE/skills"

# If built-in skills source exists (copied during image build),
# sync any missing skills into the mounted workspace volume.
if [ -d "$SKILLS_SRC" ]; then
    mkdir -p "$SKILLS_DST"
    for skill_dir in "$SKILLS_SRC"/*/; do
        skill_name="$(basename "$skill_dir")"
        if [ ! -d "$SKILLS_DST/$skill_name" ]; then
            cp -r "$skill_dir" "$SKILLS_DST/$skill_name"
            echo "Installed built-in skill: $skill_name"
        else
            # Always update SKILL.md from built-in to pick up metadata fixes
            if [ -f "$skill_dir/SKILL.md" ]; then
                cp "$skill_dir/SKILL.md" "$SKILLS_DST/$skill_name/SKILL.md"
            fi
        fi
    done
fi

# Ensure memory directory exists (for bind mount)
mkdir -p "$WORKSPACE/memory"

exec picoclaw "$@"
