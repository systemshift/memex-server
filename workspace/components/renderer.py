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
