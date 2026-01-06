"""
Component Renderer for Memex Workspace.

Converts tool calls (component specs) into HTML fragments.
"""

from typing import Dict, Any, List, Optional
from markupsafe import Markup, escape


class ComponentRenderer:
    """Renders components to HTML"""

    def render(self, component_name: str, args: Dict[str, Any]) -> str:
        """Render a component to HTML"""
        renderer = getattr(self, f"render_{component_name}", None)
        if renderer:
            return renderer(args)
        return f"<!-- Unknown component: {component_name} -->"

    def render_form_header(self, args: Dict[str, Any]) -> str:
        """Render form header"""
        title = escape(args.get("title", ""))
        description = args.get("description", "")

        html = f'''
        <div class="form-header">
            <h2 class="form-title">{title}</h2>
            {f'<p class="form-description">{escape(description)}</p>' if description else ''}
        </div>
        '''
        return html

    def render_text_field(self, args: Dict[str, Any]) -> str:
        """Render text input field"""
        return self._render_input_field("text", args)

    def render_textarea_field(self, args: Dict[str, Any]) -> str:
        """Render textarea field"""
        name = escape(args.get("name", ""))
        label = escape(args.get("label", ""))
        value = escape(args.get("value", ""))
        hint = args.get("hint", "")
        required = args.get("required", False)
        done = args.get("done", False)
        rows = args.get("rows", 4)

        status_class = "done" if done else "pending"
        status_icon = "✓" if done else "?"

        html = f'''
        <div class="field-group {status_class}">
            <label class="field-label">
                <span class="status-icon">{status_icon}</span>
                {label}
                {'<span class="required">*</span>' if required else ''}
            </label>
            <textarea
                name="{name}"
                class="field-input"
                rows="{rows}"
                placeholder="{escape(hint)}"
                {'required' if required else ''}
            >{value}</textarea>
            {f'<span class="field-hint">{escape(hint)}</span>' if hint and not value else ''}
        </div>
        '''
        return html

    def render_currency_field(self, args: Dict[str, Any]) -> str:
        """Render currency field"""
        name = escape(args.get("name", ""))
        label = escape(args.get("label", ""))
        value = args.get("value", "")
        currency = args.get("currency", "$")
        hint = args.get("hint", "")
        required = args.get("required", False)
        done = args.get("done", False)

        status_class = "done" if done else "pending"
        status_icon = "✓" if done else "?"

        html = f'''
        <div class="field-group {status_class}">
            <label class="field-label">
                <span class="status-icon">{status_icon}</span>
                {label}
                {'<span class="required">*</span>' if required else ''}
            </label>
            <div class="currency-input">
                <span class="currency-symbol">{escape(currency)}</span>
                <input
                    type="number"
                    name="{name}"
                    class="field-input"
                    value="{value if value else ''}"
                    placeholder="{escape(hint)}"
                    step="0.01"
                    {'required' if required else ''}
                >
            </div>
            {f'<span class="field-hint">{escape(hint)}</span>' if hint and not value else ''}
            {self._render_auto_filled_badge(args) if done and value else ''}
        </div>
        '''
        return html

    def render_date_field(self, args: Dict[str, Any]) -> str:
        """Render date field"""
        return self._render_input_field("date", args)

    def render_email_field(self, args: Dict[str, Any]) -> str:
        """Render email field"""
        return self._render_input_field("email", args)

    def render_select_field(self, args: Dict[str, Any]) -> str:
        """Render select dropdown field"""
        name = escape(args.get("name", ""))
        label = escape(args.get("label", ""))
        options = args.get("options", [])
        value = args.get("value", "")
        hint = args.get("hint", "")
        required = args.get("required", False)
        done = args.get("done", False)

        status_class = "done" if done else "pending"
        status_icon = "✓" if done else "?"

        options_html = '<option value="">Select...</option>'
        for opt in options:
            selected = 'selected' if opt == value else ''
            options_html += f'<option value="{escape(opt)}" {selected}>{escape(opt)}</option>'

        html = f'''
        <div class="field-group {status_class}">
            <label class="field-label">
                <span class="status-icon">{status_icon}</span>
                {label}
                {'<span class="required">*</span>' if required else ''}
            </label>
            <select name="{name}" class="field-input" {'required' if required else ''}>
                {options_html}
            </select>
            {f'<span class="field-hint">{escape(hint)}</span>' if hint and not value else ''}
        </div>
        '''
        return html

    def render_file_field(self, args: Dict[str, Any]) -> str:
        """Render file upload field"""
        name = escape(args.get("name", ""))
        label = escape(args.get("label", ""))
        accept = args.get("accept", "*/*")
        hint = args.get("hint", "")
        required = args.get("required", False)
        done = args.get("done", False)

        status_class = "done" if done else "pending"
        status_icon = "✓" if done else "?"

        html = f'''
        <div class="field-group {status_class}">
            <label class="field-label">
                <span class="status-icon">{status_icon}</span>
                {label}
                {'<span class="required">*</span>' if required else ''}
            </label>
            <div class="file-upload">
                <input
                    type="file"
                    name="{name}"
                    accept="{escape(accept)}"
                    class="file-input"
                    {'required' if required else ''}
                >
                <span class="file-label">Choose file or drag here</span>
            </div>
            {f'<span class="field-hint">{escape(hint)}</span>' if hint else ''}
        </div>
        '''
        return html

    def render_checkbox_field(self, args: Dict[str, Any]) -> str:
        """Render checkbox field"""
        name = escape(args.get("name", ""))
        label = escape(args.get("label", ""))
        checked = args.get("checked", False)
        required = args.get("required", False)

        html = f'''
        <div class="field-group checkbox-group">
            <label class="checkbox-label">
                <input
                    type="checkbox"
                    name="{name}"
                    {'checked' if checked else ''}
                    {'required' if required else ''}
                >
                <span>{label}</span>
            </label>
        </div>
        '''
        return html

    def render_context_card(self, args: Dict[str, Any]) -> str:
        """Render context card from Memex"""
        title = escape(args.get("title", ""))
        content = escape(args.get("content", ""))
        card_type = args.get("type", "info")

        icon_map = {
            "info": "💡",
            "policy": "📋",
            "related": "🔗",
            "warning": "⚠️"
        }
        icon = icon_map.get(card_type, "📋")

        html = f'''
        <div class="context-card context-{card_type}">
            <div class="context-icon">{icon}</div>
            <div class="context-content">
                <div class="context-title">{title}</div>
                <div class="context-text">{content}</div>
            </div>
        </div>
        '''
        return html

    def render_action_bar(self, args: Dict[str, Any]) -> str:
        """Render action buttons"""
        primary = escape(args.get("primary_action", "Submit"))
        secondary = args.get("secondary_actions", [])
        disabled = args.get("primary_disabled", False)

        secondary_html = ""
        for action in secondary:
            secondary_html += f'<button type="button" class="btn btn-secondary">{escape(action)}</button>'

        html = f'''
        <div class="action-bar">
            <button type="submit" class="btn btn-primary" {'disabled' if disabled else ''}>
                {primary}
            </button>
            {secondary_html}
        </div>
        '''
        return html

    def render_text_display(self, args: Dict[str, Any]) -> str:
        """Render text display"""
        content = escape(args.get("content", ""))
        style = args.get("style", "normal")

        tag_map = {
            "heading": "h3",
            "subheading": "h4",
            "normal": "p",
            "muted": "p"
        }
        tag = tag_map.get(style, "p")
        class_name = f"text-{style}"

        return f'<{tag} class="{class_name}">{content}</{tag}>'

    def render_divider(self, args: Dict[str, Any]) -> str:
        """Render divider"""
        label = args.get("label", "")
        if label:
            return f'<div class="divider"><span>{escape(label)}</span></div>'
        return '<hr class="divider">'

    def _render_input_field(self, field_type: str, args: Dict[str, Any]) -> str:
        """Render generic input field"""
        name = escape(args.get("name", ""))
        label = escape(args.get("label", ""))
        value = escape(str(args.get("value", ""))) if args.get("value") else ""
        hint = args.get("hint", "")
        required = args.get("required", False)
        done = args.get("done", False)

        status_class = "done" if done else "pending"
        status_icon = "✓" if done else "?"

        html = f'''
        <div class="field-group {status_class}">
            <label class="field-label">
                <span class="status-icon">{status_icon}</span>
                {label}
                {'<span class="required">*</span>' if required else ''}
            </label>
            <input
                type="{field_type}"
                name="{name}"
                class="field-input"
                value="{value}"
                placeholder="{escape(hint)}"
                {'required' if required else ''}
            >
            {f'<span class="field-hint">{escape(hint)}</span>' if hint and not value else ''}
            {self._render_auto_filled_badge(args) if done and value else ''}
        </div>
        '''
        return html

    def _render_auto_filled_badge(self, args: Dict[str, Any]) -> str:
        """Render auto-filled indicator"""
        return '<span class="auto-filled-badge">auto-filled (click to edit)</span>'

    # ============================================
    # Multi-User Workflow Components
    # ============================================

    def render_checklist(self, args: Dict[str, Any]) -> str:
        """Render a checklist with checkable items"""
        name = escape(args.get("name", ""))
        label = escape(args.get("label", ""))
        items = args.get("items", [])

        items_html = ""
        for item in items:
            item_id = escape(item.get("id", ""))
            item_text = escape(item.get("text", ""))
            checked = item.get("checked", False)
            required = item.get("required", False)

            items_html += f'''
            <div class="checklist-item">
                <label class="checkbox-label">
                    <input
                        type="checkbox"
                        name="{name}[]"
                        value="{item_id}"
                        {'checked' if checked else ''}
                    >
                    <span class="{'required' if required else ''}">{item_text}</span>
                </label>
            </div>
            '''

        html = f'''
        <div class="checklist-group">
            <div class="checklist-header">{label}</div>
            <div class="checklist-items">{items_html}</div>
        </div>
        '''
        return html

    def render_handoff_form(self, args: Dict[str, Any]) -> str:
        """Render handoff form (handled client-side mostly)"""
        title = escape(args.get("title", "Forward to Team"))
        context_summary = escape(args.get("context_summary", ""))

        html = f'''
        <div class="handoff-section">
            <h4 class="handoff-title">{title}</h4>
            {f'<p class="handoff-context">{context_summary}</p>' if context_summary else ''}
            <div class="handoff-form-placeholder" data-available-users='{args.get("available_users", [])}'>
                <!-- Client-side rendered -->
            </div>
        </div>
        '''
        return html

    def render_handoff_chain(self, args: Dict[str, Any]) -> str:
        """Render handoff chain visualization"""
        chain = args.get("chain", [])
        title = args.get("title", "Work History")

        if not chain:
            return ""

        steps_html = ""
        for i, step in enumerate(chain):
            user_name = escape(step.get("user_name", "Unknown"))
            user_role = escape(step.get("user_role", ""))
            stage = escape(step.get("stage", ""))
            timestamp = escape(step.get("timestamp", ""))

            steps_html += f'''
            <div class="chain-step">
                <div class="chain-user">
                    <div class="chain-avatar">{user_name[0]}</div>
                    <div class="chain-info">
                        <span class="chain-name">{user_name}</span>
                        <span class="chain-role">{user_role}</span>
                    </div>
                </div>
                {f'<div class="chain-arrow">→</div>' if i < len(chain) - 1 else ''}
            </div>
            '''

        html = f'''
        <div class="handoff-chain-display">
            <div class="chain-title">{escape(title)}</div>
            <div class="chain-steps">{steps_html}</div>
        </div>
        '''
        return html

    def render_notification_badge(self, args: Dict[str, Any]) -> str:
        """Render notification badge"""
        count = args.get("count", 0)
        badge_type = args.get("type", "info")

        if count == 0:
            return ""

        html = f'''
        <span class="notification-badge badge-{badge_type}">{count}</span>
        '''
        return html

    def render_user_avatar(self, args: Dict[str, Any]) -> str:
        """Render user avatar"""
        name = escape(args.get("name", ""))
        role = escape(args.get("role", ""))
        size = args.get("size", "medium")

        html = f'''
        <div class="user-avatar avatar-{size} {role}">
            <span class="avatar-letter">{name[0] if name else '?'}</span>
            <span class="avatar-name">{name}</span>
        </div>
        '''
        return html

    def render_status_badge(self, args: Dict[str, Any]) -> str:
        """Render status badge"""
        status = escape(args.get("status", "pending"))
        label = escape(args.get("label", status.replace("_", " ").title()))

        html = f'''
        <span class="status-badge status-{status}">{label}</span>
        '''
        return html

    def render_activity_item(self, args: Dict[str, Any]) -> str:
        """Render activity item"""
        user_name = escape(args.get("user_name", ""))
        action = escape(args.get("action", ""))
        target = escape(args.get("target", ""))
        timestamp = escape(args.get("timestamp", ""))

        html = f'''
        <div class="activity-item">
            <div class="activity-user">{user_name}</div>
            <div class="activity-action">{action}</div>
            {f'<div class="activity-target">{target}</div>' if target else ''}
            <div class="activity-time">{timestamp}</div>
        </div>
        '''
        return html

    def render_anchor_highlight(self, args: Dict[str, Any]) -> str:
        """Render anchor/entity highlight"""
        text = escape(args.get("text", ""))
        anchor_type = escape(args.get("type", ""))
        confidence = args.get("confidence", 1.0)
        properties = args.get("properties", {})

        props_html = ""
        for key, value in properties.items():
            props_html += f'<span class="anchor-prop">{escape(key)}: {escape(str(value))}</span>'

        html = f'''
        <span class="anchor-highlight anchor-{anchor_type}" title="Confidence: {confidence:.0%}">
            <span class="anchor-text">{text}</span>
            <span class="anchor-type">{anchor_type}</span>
            {f'<div class="anchor-props">{props_html}</div>' if props_html else ''}
        </span>
        '''
        return html

    def render_stats_card(self, args: Dict[str, Any]) -> str:
        """Render stats card"""
        value = escape(str(args.get("value", "")))
        label = escape(args.get("label", ""))
        change = args.get("change", "")
        trend = args.get("trend", "neutral")

        change_html = ""
        if change:
            trend_class = f"trend-{trend}"
            change_html = f'<span class="stat-change {trend_class}">{escape(change)}</span>'

        html = f'''
        <div class="stats-card">
            <div class="stat-value">{value}</div>
            <div class="stat-label">{label}</div>
            {change_html}
        </div>
        '''
        return html


# Global renderer instance
renderer = ComponentRenderer()


def render_component(name: str, args: Dict[str, Any]) -> str:
    """Render a component to HTML"""
    return renderer.render(name, args)


def render_components(components: List[Dict[str, Any]]) -> str:
    """Render multiple components to HTML"""
    html_parts = []
    for comp in components:
        name = comp.get("name", comp.get("component_type", ""))
        args = comp.get("arguments", comp.get("props", comp))
        html_parts.append(render_component(name, args))
    return "\n".join(html_parts)
