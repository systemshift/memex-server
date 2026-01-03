#!/usr/bin/env python3
"""
Generate synthetic organization data for world model testing.

Creates a realistic-ish organization with:
- Explicit relationships (we tell the model)
- Hidden patterns (model should discover)

The hidden patterns are our ground truth for evaluation.
"""

import json
import random
import hashlib
from datetime import datetime, timedelta
from typing import Dict, List, Any, Tuple
from dataclasses import dataclass, asdict

# Seed for reproducibility
random.seed(42)

# =============================================================================
# Configuration
# =============================================================================

NUM_PEOPLE = 50
NUM_TEAMS = 5
NUM_PROJECTS = 15
NUM_DOCUMENTS = 100
NUM_SKILLS = 20

SKILLS = [
    "python", "golang", "rust", "javascript", "typescript",
    "react", "vue", "kubernetes", "docker", "aws",
    "ml", "data-science", "nlp", "cv", "llm",
    "backend", "frontend", "devops", "security", "architecture"
]

TEAMS = ["Platform", "ML", "Product", "Infrastructure", "Research"]

SENIORITY = ["junior", "mid", "senior", "lead", "principal"]

DOC_TYPES = ["spec", "design", "research", "runbook", "postmortem", "rfc"]

# =============================================================================
# Data Generation
# =============================================================================

@dataclass
class Person:
    id: str
    name: str
    team: str
    seniority: str
    skills: List[str]
    joined: str

@dataclass
class Project:
    id: str
    name: str
    team: str
    status: str
    members: List[str]
    started: str

@dataclass
class Document:
    id: str
    title: str
    doc_type: str
    author: str
    project: str
    topics: List[str]
    created: str

@dataclass
class HiddenPattern:
    """Ground truth patterns the model should discover."""
    pattern_type: str
    description: str
    entities: List[str]
    strength: float  # How strong the pattern is (for evaluation)


def generate_name(i: int) -> str:
    """Generate a plausible name."""
    first_names = ["Alex", "Sam", "Jordan", "Taylor", "Morgan", "Casey",
                   "Riley", "Quinn", "Avery", "Jamie", "Drew", "Blake",
                   "Reese", "Skyler", "Peyton", "Cameron", "Dakota", "Emery"]
    last_names = ["Smith", "Chen", "Patel", "Kim", "Garcia", "Mueller",
                  "Tanaka", "Silva", "Anderson", "Lee", "Wilson", "Brown"]
    return f"{random.choice(first_names)} {random.choice(last_names)}"


def generate_people() -> List[Person]:
    """Generate people with skills and team assignments."""
    people = []
    base_date = datetime(2020, 1, 1)

    for i in range(NUM_PEOPLE):
        # Assign to team (roughly equal distribution)
        team = TEAMS[i % NUM_TEAMS]

        # Skills correlate with team (but not perfectly)
        team_skill_bias = {
            "Platform": ["golang", "kubernetes", "docker", "backend"],
            "ML": ["python", "ml", "data-science", "nlp", "llm"],
            "Product": ["javascript", "typescript", "react", "frontend"],
            "Infrastructure": ["aws", "kubernetes", "devops", "security"],
            "Research": ["python", "ml", "nlp", "cv", "architecture"]
        }

        # 70% chance of team-biased skills, 30% random
        num_skills = random.randint(2, 5)
        skills = []
        for _ in range(num_skills):
            if random.random() < 0.7:
                skills.append(random.choice(team_skill_bias[team]))
            else:
                skills.append(random.choice(SKILLS))
        skills = list(set(skills))  # Dedupe

        # Seniority correlates with join date
        days_ago = random.randint(30, 1500)
        joined = base_date + timedelta(days=1500 - days_ago)

        if days_ago > 1000:
            seniority = random.choice(["senior", "lead", "principal"])
        elif days_ago > 500:
            seniority = random.choice(["mid", "senior", "lead"])
        else:
            seniority = random.choice(["junior", "mid", "senior"])

        people.append(Person(
            id=f"person:{i:03d}",
            name=generate_name(i),
            team=team,
            seniority=seniority,
            skills=skills,
            joined=joined.isoformat()
        ))

    return people


def generate_projects(people: List[Person]) -> List[Project]:
    """Generate projects with team ownership and members."""
    projects = []
    base_date = datetime(2023, 1, 1)

    project_names = [
        "Atlas", "Beacon", "Catalyst", "Delta", "Echo",
        "Forge", "Gateway", "Horizon", "Impulse", "Junction",
        "Keystone", "Lighthouse", "Matrix", "Nexus", "Orbit"
    ]

    for i in range(NUM_PROJECTS):
        team = TEAMS[i % NUM_TEAMS]

        # Get people from this team + some cross-team
        team_people = [p for p in people if p.team == team]
        other_people = [p for p in people if p.team != team]

        # 3-7 members, mostly from same team
        num_members = random.randint(3, 7)
        num_team_members = int(num_members * 0.7)
        num_cross_team = num_members - num_team_members

        members = (
            random.sample(team_people, min(num_team_members, len(team_people))) +
            random.sample(other_people, min(num_cross_team, len(other_people)))
        )
        member_ids = [m.id for m in members]

        started = base_date + timedelta(days=random.randint(0, 300))

        projects.append(Project(
            id=f"project:{project_names[i].lower()}",
            name=project_names[i],
            team=team,
            status=random.choice(["planning", "active", "active", "active", "maintenance"]),
            members=member_ids,
            started=started.isoformat()
        ))

    return projects


def generate_documents(people: List[Person], projects: List[Project]) -> List[Document]:
    """Generate documents linked to people and projects."""
    documents = []
    base_date = datetime(2023, 6, 1)

    for i in range(NUM_DOCUMENTS):
        project = random.choice(projects)

        # Author is usually a project member
        if random.random() < 0.8 and project.members:
            author_id = random.choice(project.members)
        else:
            author_id = random.choice(people).id

        author = next(p for p in people if p.id == author_id)

        doc_type = random.choice(DOC_TYPES)

        # Topics from author's skills + project-related
        topics = random.sample(author.skills, min(2, len(author.skills)))
        if random.random() < 0.5:
            topics.append(project.name.lower())

        created = base_date + timedelta(days=random.randint(0, 180))

        title = f"{doc_type.upper()}: {project.name} - {random.choice(topics)}"

        documents.append(Document(
            id=f"doc:{i:03d}",
            title=title,
            doc_type=doc_type,
            author=author_id,
            project=project.id,
            topics=topics,
            created=created.isoformat()
        ))

    return documents


def find_hidden_patterns(
    people: List[Person],
    projects: List[Project],
    documents: List[Document]
) -> List[HiddenPattern]:
    """
    Identify hidden patterns in the data that we DON'T make explicit.
    These are the ground truth for model evaluation.
    """
    patterns = []

    # Pattern 1: Skill Clusters
    # People with same skills should be similar even if not on same team
    skill_groups: Dict[str, List[str]] = {}
    for person in people:
        for skill in person.skills:
            if skill not in skill_groups:
                skill_groups[skill] = []
            skill_groups[skill].append(person.id)

    for skill, person_ids in skill_groups.items():
        if len(person_ids) >= 3:
            patterns.append(HiddenPattern(
                pattern_type="skill_cluster",
                description=f"People sharing skill: {skill}",
                entities=person_ids,
                strength=len(person_ids) / NUM_PEOPLE
            ))

    # Pattern 2: Cross-Team Bridges
    # People who work on projects outside their team are bridges
    bridges = []
    for person in people:
        person_projects = [p for p in projects if person.id in p.members]
        cross_team = [p for p in person_projects if p.team != person.team]
        if len(cross_team) >= 1:
            bridges.append(person.id)

    if bridges:
        patterns.append(HiddenPattern(
            pattern_type="cross_team_bridge",
            description="People who bridge multiple teams",
            entities=bridges,
            strength=len(bridges) / NUM_PEOPLE
        ))

    # Pattern 3: Document Co-authorship Affinity
    # People who write about same topics (even on different projects)
    topic_authors: Dict[str, List[str]] = {}
    for doc in documents:
        for topic in doc.topics:
            if topic not in topic_authors:
                topic_authors[topic] = []
            if doc.author not in topic_authors[topic]:
                topic_authors[topic].append(doc.author)

    for topic, authors in topic_authors.items():
        if len(authors) >= 3:
            patterns.append(HiddenPattern(
                pattern_type="topic_affinity",
                description=f"Authors writing about: {topic}",
                entities=authors,
                strength=len(authors) / NUM_PEOPLE
            ))

    # Pattern 4: Project Dependency (implicit through shared members)
    # If same people work on two projects, they're likely related
    project_pairs = []
    for i, p1 in enumerate(projects):
        for p2 in projects[i+1:]:
            shared = set(p1.members) & set(p2.members)
            if len(shared) >= 2:
                project_pairs.append({
                    "projects": [p1.id, p2.id],
                    "shared_members": list(shared),
                    "strength": len(shared) / min(len(p1.members), len(p2.members))
                })

    for pair in project_pairs:
        patterns.append(HiddenPattern(
            pattern_type="project_dependency",
            description=f"Projects sharing members: {pair['projects']}",
            entities=pair['projects'] + pair['shared_members'],
            strength=pair['strength']
        ))

    # Pattern 5: Knowledge Silos
    # Some topics only appear in one team's documents
    team_topics: Dict[str, set] = {t: set() for t in TEAMS}
    for doc in documents:
        project = next(p for p in projects if p.id == doc.project)
        for topic in doc.topics:
            team_topics[project.team].add(topic)

    for team, topics in team_topics.items():
        unique_topics = topics - set().union(*[t for t2, t in team_topics.items() if t2 != team])
        if unique_topics:
            patterns.append(HiddenPattern(
                pattern_type="knowledge_silo",
                description=f"Topics unique to {team}: {unique_topics}",
                entities=[team] + list(unique_topics),
                strength=len(unique_topics) / len(topics) if topics else 0
            ))

    return patterns


def generate_explicit_links(
    people: List[Person],
    projects: List[Project],
    documents: List[Document]
) -> List[Dict[str, Any]]:
    """
    Generate explicit links that we WILL tell the model about.
    """
    links = []

    # Person -> Team
    for person in people:
        links.append({
            "source": person.id,
            "target": f"team:{person.team.lower()}",
            "type": "BELONGS_TO",
            "meta": {}
        })

    # Person -> Project (membership)
    for project in projects:
        for member_id in project.members:
            links.append({
                "source": member_id,
                "target": project.id,
                "type": "WORKS_ON",
                "meta": {}
            })

    # Project -> Team
    for project in projects:
        links.append({
            "source": project.id,
            "target": f"team:{project.team.lower()}",
            "type": "OWNED_BY",
            "meta": {}
        })

    # Document -> Author
    for doc in documents:
        links.append({
            "source": doc.id,
            "target": doc.author,
            "type": "AUTHORED_BY",
            "meta": {}
        })

    # Document -> Project
    for doc in documents:
        links.append({
            "source": doc.id,
            "target": doc.project,
            "type": "ABOUT",
            "meta": {}
        })

    return links


def generate_attention_edges(
    people: List[Person],
    projects: List[Project],
    documents: List[Document]
) -> List[Dict[str, Any]]:
    """
    Generate simulated attention edges (as if from real usage).
    These represent "what gets accessed together".
    """
    edges = []

    # Simulate: when someone views a project, they often view related docs
    for project in projects:
        project_docs = [d for d in documents if d.project == project.id]
        for doc in project_docs:
            if random.random() < 0.6:  # 60% chance of edge
                edges.append({
                    "source": project.id,
                    "target": doc.id,
                    "weight": random.uniform(0.3, 0.9),
                    "query_id": "synthetic"
                })

    # Simulate: team members view each other's work
    for person in people:
        teammates = [p for p in people if p.team == person.team and p.id != person.id]
        for teammate in random.sample(teammates, min(3, len(teammates))):
            if random.random() < 0.4:
                edges.append({
                    "source": person.id,
                    "target": teammate.id,
                    "weight": random.uniform(0.2, 0.7),
                    "query_id": "synthetic"
                })

    # Simulate: skill-based affinity (people look up others with same skills)
    for person in people:
        for skill in person.skills:
            similar = [p for p in people if skill in p.skills and p.id != person.id]
            for other in random.sample(similar, min(2, len(similar))):
                if random.random() < 0.3:
                    edges.append({
                        "source": person.id,
                        "target": other.id,
                        "weight": random.uniform(0.3, 0.6),
                        "query_id": "synthetic"
                    })

    return edges


def main():
    print("Generating synthetic organization data...")

    # Generate entities
    people = generate_people()
    print(f"  Generated {len(people)} people")

    projects = generate_projects(people)
    print(f"  Generated {len(projects)} projects")

    documents = generate_documents(people, projects)
    print(f"  Generated {len(documents)} documents")

    # Generate teams as entities
    teams = [{"id": f"team:{t.lower()}", "name": t, "type": "Team"} for t in TEAMS]

    # Generate links
    links = generate_explicit_links(people, projects, documents)
    print(f"  Generated {len(links)} explicit links")

    # Generate attention edges
    attention_edges = generate_attention_edges(people, projects, documents)
    print(f"  Generated {len(attention_edges)} attention edges")

    # Find hidden patterns (ground truth)
    hidden_patterns = find_hidden_patterns(people, projects, documents)
    print(f"  Found {len(hidden_patterns)} hidden patterns (ground truth)")

    # Compile output
    output = {
        "metadata": {
            "generated_at": datetime.now().isoformat(),
            "num_people": len(people),
            "num_projects": len(projects),
            "num_documents": len(documents),
            "num_teams": len(teams),
            "num_links": len(links),
            "num_attention_edges": len(attention_edges),
            "num_hidden_patterns": len(hidden_patterns)
        },
        "entities": {
            "people": [asdict(p) for p in people],
            "projects": [asdict(p) for p in projects],
            "documents": [asdict(d) for d in documents],
            "teams": teams
        },
        "links": links,
        "attention_edges": attention_edges,
        "hidden_patterns": [asdict(p) for p in hidden_patterns]
    }

    # Write to file
    output_path = "bench/synthetic_org_data.json"
    with open(output_path, "w") as f:
        json.dump(output, f, indent=2)

    print(f"\nOutput written to {output_path}")

    # Print summary of hidden patterns
    print("\nHidden patterns (ground truth for evaluation):")
    pattern_types = {}
    for p in hidden_patterns:
        if p.pattern_type not in pattern_types:
            pattern_types[p.pattern_type] = 0
        pattern_types[p.pattern_type] += 1

    for ptype, count in pattern_types.items():
        print(f"  {ptype}: {count}")

    return output


if __name__ == "__main__":
    main()
