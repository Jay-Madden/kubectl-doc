(function(global){
  function ready(fn){ if(document.readyState !== "loading"){ fn(); } else { document.addEventListener("DOMContentLoaded", fn); } }
  function escapeHTML(value){
    return String(value || "").replace(/[&<>"']/g, function(ch){
      return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[ch];
    });
  }
  function attr(value){ return escapeHTML(value); }
  function fieldMap(schema){
    var map = new Map();
    (schema.fields || []).forEach(function(field){ map.set(field.id, field); });
    return map;
  }
  function detailHTML(field){
    if(!field){ return ""; }
    var html = "<dl class=\"kdoc-detail-grid\">";
    html += "<div class=\"kdoc-detail-row\"><dt>Path</dt><dd><code class=\"kdoc-detail-code\">" + escapeHTML(field.path) + "</code></dd></div>";
    html += "<div class=\"kdoc-detail-row\"><dt>Type</dt><dd><code class=\"kdoc-detail-code\">" + escapeHTML(field.type) + "</code></dd></div>";
    html += "<div class=\"kdoc-detail-row\"><dt>Required</dt><dd><span class=\"kdoc-detail-badge " + (field.required ? "kdoc-detail-badge-required" : "kdoc-detail-badge-optional") + "\">" + (field.required ? "yes" : "no") + "</span></dd></div>";
    html += "</dl>";
    if(field.description){
      html += "<section class=\"kdoc-detail-section\"><h3>Description</h3><p class=\"kdoc-detail-description\">" + escapeHTML(field.description) + "</p></section>";
    }
    if(field.metadata && field.metadata.length){
      html += "<section class=\"kdoc-detail-section\"><h3>Validation and metadata</h3><ul class=\"kdoc-detail-list\">";
      field.metadata.forEach(function(item){ html += "<li><code>" + escapeHTML(item) + "</code></li>"; });
      html += "</ul></section>";
    }
    return html;
  }
  function isCommentLine(text){
    return /^\s*(?:-\s*)?#/.test(text || "");
  }
  function commentParts(text){
    var match = String(text || "").match(/^(\s*(?:-\s*)?#\s?)(.*)$/);
    if(!match){ return null; }
    var prefix = match[1];
    var nextPrefix = prefix;
    if(/-\s*#/.test(prefix)){
      nextPrefix = prefix.replace(/-\s*#\s?/, "  # ");
    }
    return {prefix: prefix, nextPrefix: nextPrefix, text: match[2]};
  }
  function renderCommentText(text){
    var parts = commentParts(text);
    if(!parts){ return escapeHTML(text); }
    return "<span class=\"kdoc-comment\" data-kdoc-comment data-kdoc-comment-prefix=\"" + attr(parts.prefix) + "\" data-kdoc-comment-wrap-prefix=\"" + attr(parts.nextPrefix) + "\" data-kdoc-comment-text=\"" + attr(parts.text) + "\"><span class=\"kdoc-yaml-comment kdoc-comment-prefix\">" + escapeHTML(parts.prefix) + "</span><span class=\"kdoc-yaml-comment kdoc-comment-body\">" + escapeHTML(parts.text) + "</span></span>";
  }
  function renderPayloadComment(comment){
    var prefix = comment.prefix || comment.p || "";
    var wrapPrefix = comment.wrapPrefix || comment.w || prefix;
    var text = comment.text || comment.t || "";
    return "<span class=\"kdoc-comment\" data-kdoc-comment data-kdoc-comment-prefix=\"" + attr(prefix) + "\" data-kdoc-comment-wrap-prefix=\"" + attr(wrapPrefix) + "\" data-kdoc-comment-text=\"" + attr(text) + "\"><span class=\"kdoc-yaml-comment kdoc-comment-prefix\">" + escapeHTML(prefix) + "</span><span class=\"kdoc-yaml-comment kdoc-comment-body\">" + escapeHTML(text) + "</span></span>";
  }
  function tokenClass(kind){
    switch(kind || ""){
    case "bool":
      return "kdoc-yaml-bool";
    case "comment":
      return "kdoc-yaml-comment";
    case "key":
      return "kdoc-yaml-key";
    case "null":
      return "kdoc-yaml-null";
    case "number":
      return "kdoc-yaml-number";
    case "placeholder":
      return "kdoc-yaml-placeholder";
    case "punct":
      return "kdoc-yaml-punct";
    case "required":
      return "kdoc-required-label";
    case "scalar":
      return "kdoc-yaml-scalar";
    case "string":
      return "kdoc-yaml-string";
    case "type-number":
      return "kdoc-yaml-type-number";
    default:
      return "";
    }
  }
  function tokenText(token){
    if(token.t != null){ return token.t; }
    if(token.text != null){ return token.text; }
    return "";
  }
  function renderPayloadToken(token){
    var text = tokenText(token);
    var className = tokenClass(token.k || token.kind);
    if(!className){ return escapeHTML(text); }
    return "<span class=\"" + className + "\">" + escapeHTML(text) + "</span>";
  }
  function lineText(line){
    if(line.text != null){ return String(line.text); }
    if(line.comment){
      return String(line.comment.prefix || line.comment.p || "") + String(line.comment.text || line.comment.t || "");
    }
    if(line.tokens && line.tokens.length){
      return line.tokens.map(tokenText).join("");
    }
    return "";
  }
  function renderLineYAML(line, text){
    if(line.comment){ return renderPayloadComment(line.comment); }
    if(line.tokens && line.tokens.length){
      return line.tokens.map(renderPayloadToken).join("");
    }
    if(isCommentLine(text)){ return renderCommentText(text); }
    return escapeHTML(text);
  }
  function renderSchema(root, schema, options){
    var fields = fieldMap(schema || {});
    var filtering = options.filtering !== false;
    var showWrapControl = options.wrapControl !== false;
    var wrapChecked = options.wrapComments !== false;
    root.classList.add("kubectl-doc");
    root.classList.toggle("kdoc-details-side-overlay", options.detailsMode === "side-overlay");
    root.classList.toggle("kdoc-filter-disabled", !filtering);
    root.setAttribute("data-kubectl-doc", "");
    if(!root.hasAttribute("tabindex")){ root.setAttribute("tabindex", "0"); }
    var html = "<div class=\"kdoc-layout\"><section class=\"kdoc-docs\"><div class=\"kdoc-filter-overlay\" data-kdoc-filter-overlay hidden></div><section class=\"kdoc-version\"><div class=\"kdoc-tree\" role=\"tree\" aria-label=\"" + attr((schema && schema.kind ? schema.kind : "Kubernetes") + " YAML schema") + "\">";
    (schema.lines || []).forEach(function(line, index){
      var field = line.detailId ? fields.get(line.detailId) : null;
      var text = lineText(line);
      var classes = "kdoc-line" + (text.trim() ? "" : " kdoc-blank");
      var fieldAttr = line.field ? " data-kdoc-field data-kdoc-field-name=\"" + attr(line.field) + "\" data-kdoc-filter-text=\"" + attr((line.field || "") + "\n" + (field && field.description ? field.description : "")) + "\"" : "";
      var detailID = line.detailId || ("line-" + index);
      html += "<div class=\"" + classes + "\" role=\"treeitem\" data-kdoc-line" + fieldAttr + " data-index=\"" + attr(line.index != null ? line.index : index) + "\" data-depth=\"" + attr(line.depth || 0) + "\" data-path=\"" + attr(line.path || "") + "\" data-detail-id=\"" + attr(detailID) + "\" data-detail=\"\" data-detail-html=\"" + attr(detailHTML(field)) + "\">";
      if(line.foldable){
        html += "<button class=\"kdoc-fold\" type=\"button\" aria-label=\"Toggle\" aria-expanded=\"" + (line.collapsed ? "false" : "true") + "\" data-kdoc-toggle></button>";
      } else {
        html += "<span class=\"kdoc-gutter\"></span>";
      }
      var commentText = line.comment || (!line.tokens && isCommentLine(text));
      html += "<span class=\"kdoc-yaml-text" + (commentText ? " kdoc-yaml-comment-text" : "") + "\">" + renderLineYAML(line, text) + "</span>";
      html += "</div>";
    });
    html += "</div></section></section><aside class=\"kdoc-details\" data-kdoc-details aria-live=\"polite\"><h2>Details</h2><div class=\"kdoc-detail-body\" data-kdoc-detail-body><p class=\"kdoc-detail-empty\">Select a field.</p></div></aside></div>";
    if(showWrapControl){
      html += "<div class=\"kdoc-view-controls\" aria-label=\"View options\"><label class=\"kdoc-wrap-toggle\"><input type=\"checkbox\" data-kdoc-wrap-comments" + (wrapChecked ? " checked" : "") + "><span class=\"kdoc-switch\" aria-hidden=\"true\"></span><span class=\"kdoc-wrap-label\">wrap</span></label></div>";
    }
    root.innerHTML = html;
  }
  function mount(root, options){
      options = options || {};
      if(!root){ return null; }
      if(root.__kubectlDocController){ return root.__kubectlDocController; }
      if(options.initialSchema && !root.querySelector("[data-kdoc-line]")){
        renderSchema(root, options.initialSchema, options);
      }

      var lines = Array.prototype.slice.call(root.querySelectorAll("[data-kdoc-line]"));
      var comments = Array.prototype.slice.call(root.querySelectorAll("[data-kdoc-comment]"));
      var details = root.querySelector("[data-kdoc-detail-body]");
      var wrapComments = root.querySelector("[data-kdoc-wrap-comments]");
      var filterOverlay = root.querySelector("[data-kdoc-filter-overlay]");
      var backURL = options.backURL || root.getAttribute("data-kdoc-back-url");
      var quitURL = options.quitURL || root.getAttribute("data-kdoc-quit-url");
      var resizeFrame = 0;
      var charWidthCache = 0;
      var commentColumnCache = 0;
      var currentLine = null;
      var filterQuery = "";
      var activeFilterState = null;
      var lineStates = [];
      var stateByLine = new Map();
      var fieldStates = [];
      var detailFieldByID = new Map();
      var detailLineGroups = new Map();
      var detailLineStates = new Map();
      var allLineSet = new Set(lines);
      var commentStates = [];
      var highlightedElements = [];
      var selectedLines = [];
      var filterVisibleLines = [];
      var filtering = options.filtering !== false;
      var loadingFullSchema = false;
      var mountedOptions = options;
      var controller = null;
      var staleBackdropTimers = [];
      var scopedKeyboard = options.detailsMode === "side-overlay";
      if(scopedKeyboard && !root.hasAttribute("tabindex")){ root.setAttribute("tabindex", "0"); }
      root.classList.toggle("kdoc-details-side-overlay", scopedKeyboard);

      lines.forEach(function(line, index){
        var detailID = line.getAttribute("data-detail-id") || "";
        var path = (line.getAttribute("data-path") || "").toLowerCase();
        var textElement = line.querySelector(".kdoc-yaml-text");
        var state = {
          line: line,
          index: index,
          depth: Number(line.getAttribute("data-depth") || "0"),
          field: line.hasAttribute("data-kdoc-field"),
          filterText: (line.getAttribute("data-kdoc-filter-text") || "").toLowerCase(),
          path: path,
          pathParts: path ? path.split(".") : [],
          detailID: detailID,
          textTrim: line.textContent.trim(),
          textElement: textElement,
          textLower: textElement ? textElement.textContent.toLowerCase() : "",
          toggle: line.querySelector("[data-kdoc-toggle]"),
          fieldState: null,
          ancestors: [],
          descendants: [],
          pathHit: ""
        };
        lineStates.push(state);
        stateByLine.set(line, state);
        if(detailID){
          if(!detailLineGroups.has(detailID)){ detailLineGroups.set(detailID, []); }
          if(!detailLineStates.has(detailID)){ detailLineStates.set(detailID, []); }
          detailLineGroups.get(detailID).push(line);
          detailLineStates.get(detailID).push(state);
        }
        if(state.field){
          state.fieldState = state;
          fieldStates.push(state);
          if(detailID && !detailFieldByID.has(detailID)){ detailFieldByID.set(detailID, state); }
        }
      });
      lineStates.forEach(function(state){
        if(!state.field && state.detailID && detailFieldByID.has(state.detailID)){
          state.fieldState = detailFieldByID.get(state.detailID);
        }
      });
      var ancestorStack = [];
      fieldStates.forEach(function(state){
        while(ancestorStack.length && ancestorStack[ancestorStack.length - 1].depth >= state.depth){ ancestorStack.pop(); }
        state.ancestors = ancestorStack.slice();
        state.ancestors.forEach(function(ancestor){ ancestor.descendants.push(state); });
        ancestorStack.push(state);
      });

      function lineState(line){ return stateByLine.get(line) || null; }
      function button(line){ var state = lineState(line); return state ? state.toggle : line.querySelector("[data-kdoc-toggle]"); }
      function depth(line){ var state = lineState(line); return state ? state.depth : Number(line.getAttribute("data-depth") || "0"); }
      comments.forEach(function(comment){
        var line = comment.closest("[data-kdoc-line]");
        commentStates.push({
          comment: comment,
          line: line,
          firstPrefix: comment.getAttribute("data-kdoc-comment-prefix") || "",
          nextPrefix: comment.getAttribute("data-kdoc-comment-wrap-prefix") || comment.getAttribute("data-kdoc-comment-prefix") || "",
          text: comment.getAttribute("data-kdoc-comment-text") || "",
          wrapState: ""
        });
      });
      function expanded(line){ var b = button(line); return !b || b.getAttribute("aria-expanded") !== "false"; }
      function setExpanded(line, value){
        var b = button(line);
        if(!b){ return; }
        b.setAttribute("aria-expanded", value ? "true" : "false");
      }
      function hasLoadedDescendants(line){
        var state = lineState(line);
        return !!(state && state.descendants && state.descendants.length);
      }
      function wantsFullSchemaForExpansion(line){
        return !!(line && mountedOptions.initialSchema && mountedOptions.initialSchema.complete === false && !expanded(line) && !hasLoadedDescendants(line));
      }
      function expandWithFullSchema(line){
        if(wantsFullSchemaForExpansion(line)){
          select(line, {scroll:false});
          setExpanded(line, true);
          requestFullSchema();
          return true;
        }
        setExpanded(line, true);
        return false;
      }
      function toggleExpandedWithFullSchema(line){
        if(!expanded(line)){
          expandWithFullSchema(line);
          return;
        }
        setExpanded(line, false);
      }
      function setLineHidden(state, value){
        if(state.line.hidden !== value){ state.line.hidden = value; }
      }
      function nextContentDepth(index){
        for(var i = index; i < lineStates.length; i++){
          if(lineStates[i].textTrim !== ""){ return lineStates[i].depth; }
        }
        return null;
      }
      function cleanPathComponent(component){
        return String(component || "").replace(/\[\]$/, "");
      }
      function cleanPathComponents(parts){
        return parts.map(cleanPathComponent);
      }
      function pathComponentEqual(component, token){
        return component === token || cleanPathComponent(component) === token;
      }
      function pathComponentContains(component, token){
        return component.indexOf(token) >= 0 || cleanPathComponent(component).indexOf(token) >= 0;
      }
      function parsePathFilter(query){
        query = String(query || "").toLowerCase();
        var anchored = query.indexOf(".") === 0 && query.indexOf("...") !== 0;
        if(anchored){ query = query.slice(1); }
        if(!query || (!anchored && query.indexOf(".") < 0)){ return null; }

        var filter = {anchored: anchored, tokens: [], suffix: ""};
        for(var i = 0; i < query.length; ){
          if(query.slice(i, i + 3) === "..."){
            filter.tokens.push("...");
            i += 3;
            continue;
          }
          if(query[i] === "."){
            i++;
            continue;
          }

          var start = i;
          while(i < query.length && query[i] !== "."){ i++; }
          var token = query.slice(start, i);
          if(!token){ continue; }
          if(/\s/.test(token)){
            filter.suffix = query.slice(start);
            break;
          }
          filter.tokens.push(token);
        }
        if(!filter.tokens.length && !filter.suffix){ return null; }
        return filter;
      }
      function pathSuffixOverlapsFinalComponent(parts, suffix){
        var text = parts.join(".");
        var finalStart = text.length - parts[parts.length - 1].length;
        var offset = 0;
        while(offset <= text.length){
          var index = text.indexOf(suffix, offset);
          if(index < 0){ return false; }
          if(index + suffix.length > finalStart){ return true; }
          offset = index + 1;
        }
        return false;
      }
      function pathSuffixHighlight(parts, suffix){
        if(!parts.length){ return ""; }
        if(pathSuffixOverlapsFinalComponent(parts, suffix) || pathSuffixOverlapsFinalComponent(cleanPathComponents(parts), suffix)){
          var index = suffix.lastIndexOf(".");
          return index >= 0 ? suffix.slice(index + 1) : suffix;
        }
        return "";
      }
      function matchPathFilter(parts, partIndex, tokens, tokenIndex, suffix){
        if(tokenIndex === tokens.length){
          if(suffix){ return pathSuffixHighlight(parts.slice(partIndex), suffix); }
          return partIndex === parts.length ? "__match__" : "";
        }
        if(tokens[tokenIndex] === "..."){
          if(tokenIndex === tokens.length - 1 && !suffix){
            return cleanPathComponent(parts[parts.length - 1] || "");
          }
          for(var skip = partIndex; skip <= parts.length; skip++){
            var wildcardHit = matchPathFilter(parts, skip, tokens, tokenIndex + 1, suffix);
            if(wildcardHit){ return wildcardHit; }
          }
          return "";
        }
        if(partIndex >= parts.length){ return ""; }

        var token = tokens[tokenIndex];
        if(tokenIndex === tokens.length - 1 && !suffix){
          return partIndex === parts.length - 1 && pathComponentContains(parts[partIndex], token) ? token : "";
        }
        if(!pathComponentEqual(parts[partIndex], token)){ return ""; }
        return matchPathFilter(parts, partIndex + 1, tokens, tokenIndex + 1, suffix);
      }
      function pathFilterHighlightForState(state, filter){
        if(!filter || !state || !state.pathParts.length){ return ""; }
        var parts = state.pathParts;
        if(filter.anchored){
          var anchoredHit = matchPathFilter(parts, 0, filter.tokens, 0, filter.suffix);
          return anchoredHit === "__match__" ? "" : anchoredHit;
        }
        for(var start = 0; start < parts.length; start++){
          var hit = matchPathFilter(parts, start, filter.tokens, 0, filter.suffix);
          if(hit){ return hit === "__match__" ? "" : hit; }
        }
        return "";
      }
      function ancestorFieldLines(line){
        var state = lineState(line);
        if(!state || !state.fieldState){ return []; }
        return state.fieldState.ancestors.map(function(ancestor){ return ancestor.line; }).reverse();
      }
      function currentFilterState(){
        var query = filterQuery.toLowerCase();
        if(!query){ return null; }
        if(activeFilterState && activeFilterState.query === query){ return activeFilterState; }

        var pathFilter = parsePathFilter(query);
        var directFields = new Set();
        fieldStates.forEach(function(state){
          state.pathHit = pathFilterHighlightForState(state, pathFilter);
          if(state.filterText.indexOf(query) >= 0 || state.pathHit){
            directFields.add(state);
          }
        });

        var includedFields = new Set();
        var allowedLines = new Set();
        var highlightLineStates = new Set();
        directFields.forEach(function(state){
          includedFields.add(state);
          state.ancestors.forEach(function(ancestor){ includedFields.add(ancestor); });
          state.descendants.forEach(function(descendant){ includedFields.add(descendant); });
          groupedLineStates(state).forEach(function(lineStateValue){
            allowedLines.add(lineStateValue.line);
            highlightLineStates.add(lineStateValue);
          });
        });
        includedFields.forEach(function(state){
          groupedLineStates(state).forEach(function(lineStateValue){ allowedLines.add(lineStateValue.line); });
        });

        var directLines = new Set();
        directFields.forEach(function(state){ directLines.add(state.line); });
        activeFilterState = {
          query: query,
          pathFilter: pathFilter,
          directFields: directFields,
          directLines: directLines,
          includedFields: includedFields,
          allowedLines: allowedLines,
          highlightLineStates: highlightLineStates
        };
        return activeFilterState;
      }
      function directFilterMatches(){
        var state = currentFilterState();
        return state ? state.directLines : new Set();
      }
      function directFilterMatchLines(){
        var direct = directFilterMatches();
        return visibleFieldLines().filter(function(line){ return direct.has(line); });
      }
      function filterAllowedLines(){
        var state = currentFilterState();
        return state ? state.allowedLines : allLineSet;
      }
      function lineVisible(line){
        if(filterQuery){
          var state = currentFilterState();
          return !!(state && state.allowedLines.has(line));
        }
        return !line.hidden;
      }
      function setFilterVisibleLines(allowed){
        if(!filterQuery){
          root.classList.remove("kdoc-filtering");
          filterVisibleLines.forEach(function(line){ line.classList.remove("kdoc-filter-visible"); });
          filterVisibleLines = [];
          return;
        }
        root.classList.add("kdoc-filtering");
        filterVisibleLines.forEach(function(line){
          if(!allowed.has(line)){ line.classList.remove("kdoc-filter-visible"); }
        });
        var next = [];
        allowed.forEach(function(line){
          if(line.hidden){ line.hidden = false; }
          line.classList.add("kdoc-filter-visible");
          next.push(line);
        });
        filterVisibleLines = next;
      }
      function applyFolds(){
        if(filterQuery){
          setFilterVisibleLines(filterAllowedLines());
          applyFilterHighlights();
          return;
        }
        setFilterVisibleLines(allLineSet);
        lineStates.forEach(function(state){ setLineHidden(state, false); });
        lineStates.forEach(function(state, index){
          var line = state.line;
          if(!lineVisible(line) || expanded(line)){ return; }
          var parentDepth = state.depth;
          for(var i = index + 1; i < lines.length; i++){
            var blank = lineStates[i].textTrim === "";
            var followingDepth = blank ? nextContentDepth(i + 1) : null;
            if(blank && followingDepth !== null && followingDepth <= parentDepth){ break; }
            if(!blank && lineStates[i].depth <= parentDepth){ break; }
            setLineHidden(lineStates[i], true);
          }
        });
        applyFilterHighlights();
      }
      function groupedLines(line){
        var id = line.getAttribute("data-detail-id");
        if(!id){ return [line]; }
        return detailLineGroups.get(id) || [line];
      }
      function groupedLineStates(state){
        if(!state.detailID){ return [state]; }
        return detailLineStates.get(state.detailID) || [state];
      }
      function fieldLineFor(line){
        var state = lineState(line);
        return state && state.fieldState ? state.fieldState.line : null;
      }
      function visibleFieldLines(){
        return fieldStates.filter(function(state){ return lineVisible(state.line); }).map(function(state){ return state.line; });
      }
      function visibleFoldableLines(){
        return fieldStates.filter(function(state){ return lineVisible(state.line) && !!state.toggle; }).map(function(state){ return state.line; });
      }
      function currentFieldLine(){
        if(currentLine && lineVisible(currentLine)){ return currentLine; }
        return visibleFieldLines()[0] || null;
      }
      function lineIndex(collection, line){
        for(var i = 0; i < collection.length; i++){
          if(collection[i] === line){ return i; }
        }
        return -1;
      }
      function selectFieldByOffset(delta){
        var fields = visibleFieldLines();
        if(!fields.length){ return false; }
        var current = currentFieldLine();
        var index = lineIndex(fields, current);
        if(index < 0){ index = 0; }
        index = Math.max(0, Math.min(fields.length - 1, index + delta));
        select(fields[index], {scroll:true});
        return true;
      }
      function selectFirstField(){
        var fields = visibleFieldLines();
        if(!fields.length){ return false; }
        select(fields[0], {scroll:true});
        return true;
      }
      function selectLastField(){
        var fields = visibleFieldLines();
        if(!fields.length){ return false; }
        select(fields[fields.length - 1], {scroll:true});
        return true;
      }
      function pageFieldDistance(){
        var line = currentFieldLine();
        var height = 18;
        if(line){
          height = Math.max(line.getBoundingClientRect().height, height);
        }
        return Math.max(1, Math.floor(window.innerHeight / height / 2));
      }
      function parentField(line){
        if(!line){ return null; }
        var state = lineState(line);
        if(!state || !state.fieldState){ return null; }
        var ancestors = state.fieldState.ancestors;
        for(var i = ancestors.length - 1; i >= 0; i--){
          if(lineVisible(ancestors[i].line)){ return ancestors[i].line; }
        }
        return null;
      }
      function firstChildField(line){
        if(!line){ return null; }
        var currentDepth = depth(line);
        var fields = visibleFieldLines();
        var index = lineIndex(fields, line);
        for(var i = index + 1; i < fields.length; i++){
          if(depth(fields[i]) <= currentDepth){ return null; }
          return fields[i];
        }
        return null;
      }
      function toggleField(line){
        var toggle = button(line);
        if(!toggle){ return false; }
        toggleExpandedWithFullSchema(line);
        applyFolds();
        scheduleCommentWrap();
        select(line, {scroll:true});
        return true;
      }
      function collapseOrParent(){
        var line = currentFieldLine();
        if(!line){ return false; }
        if(button(line) && expanded(line)){
          setExpanded(line, false);
          applyFolds();
          scheduleCommentWrap();
          select(line, {scroll:true});
          return true;
        }
        var parent = parentField(line);
        if(!parent){ return false; }
        select(parent, {scroll:true});
        return true;
      }
      function expandOrChild(){
        var line = currentFieldLine();
        if(!line){ return false; }
        if(!button(line)){ return false; }
        if(!expanded(line)){
          expandWithFullSchema(line);
          applyFolds();
          scheduleCommentWrap();
          select(line, {scroll:true});
          return true;
        }
        var child = firstChildField(line);
        if(!child){ return false; }
        select(child, {scroll:true});
        return true;
      }
      function selectFoldable(delta){
        var foldable = visibleFoldableLines();
        if(!foldable.length){ return false; }
        var current = currentFieldLine();
        var index = lineIndex(foldable, current);
        if(index < 0){ index = delta > 0 ? -1 : 0; }
        index = (index + delta + foldable.length) % foldable.length;
        select(foldable[index], {scroll:true});
        return true;
      }
      function selectFilterMatch(delta){
        var matches = directFilterMatchLines();
        if(!matches.length){ return false; }
        var current = currentFieldLine();
        var index = lineIndex(matches, current);
        if(index < 0){
          index = delta > 0 ? 0 : matches.length - 1;
        } else {
          index = (index + delta + matches.length) % matches.length;
        }
        select(matches[index], {scroll:true});
        return true;
      }
      function cleanLineText(line){
        var comment = line.querySelector("[data-kdoc-comment]");
        if(comment){ return (comment.getAttribute("data-kdoc-comment-text") || "").trim(); }
        var text = line.querySelector(".kdoc-yaml-text").textContent.trim();
        if(text.indexOf("# ") === 0){ text = text.slice(2).trim(); }
        return text;
      }
      function escapeHTML(value){
        return String(value || "").replace(/[&<>"']/g, function(ch){
          return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[ch];
        });
      }
      function charWidth(){
        if(charWidthCache){ return charWidthCache; }
        var sample = document.createElement("span");
        sample.textContent = "0000000000";
        sample.style.position = "absolute";
        sample.style.visibility = "hidden";
        sample.style.whiteSpace = "pre";
        if(lines[0]){
          sample.style.font = window.getComputedStyle(lines[0]).font;
        }
        root.appendChild(sample);
        charWidthCache = Math.max(sample.getBoundingClientRect().width / 10, 1);
        sample.remove();
        return charWidthCache;
      }
      function visibleMeasureLine(){
        for(var i = 0; i < lines.length; i++){
          if(lineVisible(lines[i])){ return lines[i]; }
        }
        return lines[0] || null;
      }
      function commentLineChars(){
        if(commentColumnCache){ return commentColumnCache; }
        var line = visibleMeasureLine();
        if(!line){ return 8; }
        var gutter = line.querySelector(".kdoc-fold,.kdoc-gutter");
        var style = window.getComputedStyle(line);
        var width = line.clientWidth - parseFloat(style.paddingLeft || "0") - parseFloat(style.paddingRight || "0");
        var text = line.querySelector(".kdoc-yaml-text");
        if(text && window.getComputedStyle(text).display !== "inline"){
          width = text.clientWidth || text.getBoundingClientRect().width || width;
          gutter = null;
        }
        if(gutter){ width -= gutter.getBoundingClientRect().width; }
        commentColumnCache = Math.max(Math.floor(Math.max(width, 0) / charWidth()), 8);
        return commentColumnCache;
      }
      function splitLongWord(out, word, limit){
        while(word.length > limit){
          out.push(word.slice(0, limit));
          word = word.slice(limit);
        }
        return word;
      }
      function wrapCommentText(text, firstLimit, nextLimit){
        var words = String(text || "").trim().split(/\s+/).filter(Boolean);
        var out = [];
        var current = "";
        function limit(){ return out.length === 0 ? firstLimit : nextLimit; }
        words.forEach(function(word){
          var currentLimit = Math.max(limit(), 1);
          if(word.length > currentLimit){
            if(current){
              out.push(current);
              current = "";
              currentLimit = Math.max(limit(), 1);
            }
            word = splitLongWord(out, word, currentLimit);
            if(!word){ return; }
          }
          if(!current){
            current = word;
            return;
          }
          if(current.length + 1 + word.length <= Math.max(limit(), 1)){
            current += " " + word;
            return;
          }
          out.push(current);
          current = word;
        });
        if(current){ out.push(current); }
        return out.length ? out : [""];
      }
      function renderCommentLine(prefix, text){
        return "<span class=\"kdoc-comment-line\"><span class=\"kdoc-yaml-comment kdoc-comment-prefix\">" + escapeHTML(prefix) + "</span><span class=\"kdoc-yaml-comment kdoc-comment-body\">" + escapeHTML(text) + "</span></span>";
      }
      function renderComment(state, wrapped, lineChars){
        if(wrapped && state.line && !lineVisible(state.line)){ return false; }
        var wrapState = wrapped ? "wrap:" + lineChars : "nowrap";
        if(state.wrapState === wrapState){ return false; }
        if(!wrapped){
          state.comment.innerHTML = "<span class=\"kdoc-yaml-comment kdoc-comment-prefix\">" + escapeHTML(state.firstPrefix) + "</span><span class=\"kdoc-yaml-comment kdoc-comment-body\">" + escapeHTML(state.text) + "</span>";
          state.wrapState = wrapState;
          return true;
        }
        var firstLimit = Math.max(lineChars - state.firstPrefix.length, 8);
        var nextLimit = Math.max(lineChars - state.nextPrefix.length, 8);
        var chunks = wrapCommentText(state.text, firstLimit, nextLimit);
        state.comment.innerHTML = chunks.map(function(chunk, index){
          return renderCommentLine(index === 0 ? state.firstPrefix : state.nextPrefix, chunk);
        }).join("\n");
        state.wrapState = wrapState;
        return true;
      }
      function applyCommentWrap(){
        var wrapped = wrapComments ? wrapComments.checked : options.wrapComments !== false;
        var lineChars = wrapped ? commentLineChars() : 0;
        var changed = false;
        root.classList.toggle("kdoc-wrap-comments", wrapped);
        commentStates.forEach(function(state){
          if(renderComment(state, wrapped, lineChars)){ changed = true; }
        });
        if(changed){ applyFilterHighlights(); }
      }
      function scheduleCommentWrap(){
        var wrapped = wrapComments ? wrapComments.checked : options.wrapComments !== false;
        if(!wrapped || resizeFrame){ return; }
        resizeFrame = window.requestAnimationFrame(function(){
          resizeFrame = 0;
          applyCommentWrap();
        });
      }
      function fallbackDetail(line){
        var path = line.getAttribute("data-path");
        var text = cleanLineText(line);
        var html = "";
        if(path){
          html += "<dl class=\"kdoc-detail-grid\"><div class=\"kdoc-detail-row\"><dt>Path</dt><dd><code class=\"kdoc-detail-code\">" + escapeHTML(path) + "</code></dd></div></dl>";
        }
        if(text){
          html += "<section class=\"kdoc-detail-section\"><p class=\"kdoc-detail-description\">" + escapeHTML(text) + "</p></section>";
        }
        return html || "<p class=\"kdoc-detail-empty\">No field details.</p>";
      }
      function showDetails(line){
        if(details){
          var detailHTML = line.getAttribute("data-detail-html");
          if(detailHTML){
            details.innerHTML = detailHTML;
          } else {
            details.innerHTML = fallbackDetail(line);
          }
        }
      }
      function updateFilterOverlay(){
        if(!filterOverlay){ return; }
        if(!filterQuery){
          filterOverlay.hidden = true;
          filterOverlay.textContent = "";
          return;
        }
        filterOverlay.hidden = false;
        filterOverlay.textContent = "filter: " + filterQuery;
      }
      function expandAncestors(line){
        ancestorFieldLines(line).forEach(function(ancestor){ setExpanded(ancestor, true); });
      }
      function clearFilter(){
        var line = currentLine;
        filterQuery = "";
        activeFilterState = null;
        updateFilterOverlay();
        if(line){ expandAncestors(line); }
        applyFolds();
        scheduleCommentWrap();
        select(line || visibleFieldLines()[0] || lines[0], {scroll:true});
      }
      function acceptFilter(){
        var line = currentLine;
        visibleFieldLines().forEach(function(field){ expandAncestors(field); });
        filterQuery = "";
        activeFilterState = null;
        updateFilterOverlay();
        applyFolds();
        scheduleCommentWrap();
        select(line || visibleFieldLines()[0] || lines[0], {scroll:true});
      }
      function ensureFilteredFocus(){
        if(currentLine && lineVisible(currentLine)){
          select(currentLine, {scroll:true});
          return;
        }
        select(visibleFieldLines()[0] || lines[0], {scroll:true});
      }
      function setFilter(value){
        filterQuery = value;
        activeFilterState = null;
        if(filtering && filterQuery && mountedOptions.initialSchema && mountedOptions.initialSchema.complete === false){
          requestFullSchema();
        }
        updateFilterOverlay();
        applyFolds();
        ensureFilteredFocus();
      }
      function filterKey(event){
        if(!filtering){ return ""; }
        if(event.key === "/" || event.key.length !== 1){ return ""; }
        if(event.key < " " || event.key === "\x7f"){ return ""; }
        return event.key;
      }
      function clearFilterHighlights(){
        highlightedElements.forEach(function(element){
          if(!element){ return; }
          Array.prototype.slice.call(element.querySelectorAll("mark.kdoc-filter-hit")).forEach(function(mark){
            mark.replaceWith(document.createTextNode(mark.textContent || ""));
          });
          element.normalize();
        });
        highlightedElements = [];
      }
      function highlightTextNode(node, query, needle){
        var value = node.nodeValue || "";
        var lower = value.toLowerCase();
        var index = lower.indexOf(needle);
        if(index < 0){ return; }
        var fragment = document.createDocumentFragment();
        var remaining = value;
        var remainingLower = lower;
        while(index >= 0){
          if(index > 0){ fragment.appendChild(document.createTextNode(remaining.slice(0, index))); }
          var hit = document.createElement("mark");
          hit.className = "kdoc-filter-hit";
          hit.textContent = remaining.slice(index, index + query.length);
          fragment.appendChild(hit);
          remaining = remaining.slice(index + query.length);
          remainingLower = remainingLower.slice(index + query.length);
          index = remainingLower.indexOf(needle);
        }
        if(remaining){ fragment.appendChild(document.createTextNode(remaining)); }
        node.replaceWith(fragment);
      }
      function highlightElement(element, query){
        var needle = query.toLowerCase();
        var walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT, {
          acceptNode: function(node){
            if(!node.nodeValue || node.parentElement.closest("mark.kdoc-filter-hit")){ return NodeFilter.FILTER_REJECT; }
            return node.nodeValue.toLowerCase().indexOf(needle) >= 0 ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
          }
        });
        var nodes = [];
        while(walker.nextNode()){ nodes.push(walker.currentNode); }
        nodes.forEach(function(node){ highlightTextNode(node, query, needle); });
        if(nodes.length && highlightedElements.indexOf(element) < 0){ highlightedElements.push(element); }
      }
      function highlightElementIfContains(element, textLower, query){
        if(!element || !query || textLower.indexOf(query) < 0){ return; }
        highlightElement(element, query);
      }
      function applyFilterHighlights(){
        clearFilterHighlights();
        if(!filterQuery){ return; }
        var query = filterQuery.toLowerCase();
        var filterState = currentFilterState();
        if(!filterState){ return; }
        filterState.highlightLineStates.forEach(function(state){
          if(!filterState.allowedLines.has(state.line)){ return; }
          var text = state.textElement;
          if(!text){ return; }
          highlightElementIfContains(text, state.textLower, query);
          var fieldState = state.fieldState || state;
          var pathHit = fieldState.pathHit || "";
          if(pathHit && state.textLower.indexOf(pathHit.toLowerCase()) >= 0){ highlightElement(text, pathHit); }
        });
      }
      function clearSelection(){
        root.querySelectorAll(".kdoc-selected").forEach(function(item){ item.classList.remove("kdoc-selected"); });
        selectedLines = [];
      }
      function select(line, options){
        if(!line){ return; }
        options = options || {};
        var fieldLine = fieldLineFor(line);
        if(fieldLine){
          line = fieldLine;
          currentLine = fieldLine;
        }
        clearSelection();
        selectedLines = groupedLines(line);
        selectedLines.forEach(function(item){ item.classList.add("kdoc-selected"); });
        showDetails(line);
        if(options.scroll && line.scrollIntoView){
          line.scrollIntoView({block:"nearest", inline:"nearest"});
        }
      }
      function typingTarget(target){
        return !!(target && (target.closest("input,textarea,select") || target.isContentEditable));
      }
      function requestQuit(){
        if(!quitURL){ return false; }
        try {
          if(navigator.sendBeacon){
            navigator.sendBeacon(quitURL, "");
          } else {
            fetch(quitURL, {method:"POST", keepalive:true}).catch(function(){});
          }
        } catch(_err) {}
        return true;
      }
      function requestFullSchema(){
        if(loadingFullSchema || !mountedOptions.loadFullSchema){ return false; }
        loadingFullSchema = true;
        var currentPath = currentLine ? currentLine.getAttribute("data-path") || "" : "";
        var currentFilter = filterQuery;
        var foldStates = [];
        fieldStates.forEach(function(state){
          if(button(state.line)){
            foldStates.push({path: state.path, expanded: expanded(state.line)});
          }
        });
        Promise.resolve(mountedOptions.loadFullSchema()).then(function(schema){
          if(!schema){ return; }
          if(controller){ controller.destroy(); }
          var nextOptions = {};
          Object.keys(mountedOptions).forEach(function(key){ nextOptions[key] = mountedOptions[key]; });
          nextOptions.initialSchema = schema;
          nextOptions.loadFullSchema = null;
          root.innerHTML = "";
          var nextController = global.KubectlDoc.mount(root, nextOptions);
          foldStates.forEach(function(item){
            if(!item.path){ return; }
            if(item.expanded && nextController && nextController.expandPath){ nextController.expandPath(item.path); }
            if(!item.expanded && nextController && nextController.collapsePath){ nextController.collapsePath(item.path); }
          });
          if(currentFilter && nextController && nextController.setFilter){ nextController.setFilter(currentFilter); }
          if(currentPath && nextController && nextController.focusPath){ nextController.focusPath(currentPath, {scroll:false}); }
        }).catch(function(error){
          loadingFullSchema = false;
          if(global.console && console.error){ console.error("kubectl-doc schema failed to load", error); }
        });
        return true;
      }
      function handleCursorKey(event){
        if(event.defaultPrevented || typingTarget(event.target)){ return false; }
        if(event.altKey || event.ctrlKey || event.metaKey){ return false; }
        var handled = false;
        if(event.key === "Escape" && filterQuery){
          clearFilter();
          handled = true;
        } else if(event.key === "Enter" && filterQuery){
          acceptFilter();
          handled = true;
        } else if(filtering && event.key === "Backspace" && filterQuery){
          setFilter(filterQuery.slice(0, -1));
          handled = true;
        } else {
          var typed = filterKey(event);
          if(filtering && typed){
            setFilter(filterQuery + typed);
            handled = true;
          }
        }
        if(!handled){ switch(event.key){
        case "ArrowUp":
          handled = selectFieldByOffset(-1);
          break;
        case "ArrowDown":
          handled = selectFieldByOffset(1);
          break;
        case "ArrowLeft":
          handled = collapseOrParent();
          break;
        case "ArrowRight":
          handled = expandOrChild();
          break;
        case "Enter":
          handled = toggleField(currentFieldLine());
          break;
        case "Tab":
          handled = filterQuery ? selectFilterMatch(event.shiftKey ? -1 : 1) : selectFoldable(event.shiftKey ? -1 : 1);
          break;
        case "Home":
          handled = selectFirstField();
          break;
        case "End":
          handled = selectLastField();
          break;
        case "PageUp":
          handled = selectFieldByOffset(-pageFieldDistance());
          break;
        case "PageDown":
          handled = selectFieldByOffset(pageFieldDistance());
          break;
        case "Escape":
          if(backURL){
            window.location.href = backURL;
            handled = true;
          } else if(requestQuit()){
            handled = true;
          }
          break;
        } }
        if(handled){
          event.preventDefault();
          event.stopPropagation();
        }
        return handled;
      }

      function handleRootClick(event){
        if(scopedKeyboard && root.focus){ root.focus({preventScroll:true}); }
        var toggle = event.target.closest("[data-kdoc-toggle]");
        if(toggle){
          var line = toggle.closest("[data-kdoc-line]");
          toggleField(line);
          return;
        }
        var line = event.target.closest("[data-kdoc-line]");
        if(line){ select(line); }
      }
      function handleWrapChange(){
        applyCommentWrap();
      }
      function handleResize(){
        commentColumnCache = 0;
        scheduleCommentWrap();
      }
      function handleFocusIn(){
        root.classList.add("kdoc-has-focus");
      }
      function handleFocusOut(event){
        var next = event.relatedTarget;
        if(!next || !root.contains(next)){ root.classList.remove("kdoc-has-focus"); }
      }
      function elementVisible(element){
        if(!element){ return false; }
        var style = global.getComputedStyle ? global.getComputedStyle(element) : null;
        if(style && (style.display === "none" || style.visibility === "hidden" || style.opacity === "0")){ return false; }
        var rect = element.getBoundingClientRect ? element.getBoundingClientRect() : null;
        return !rect || (rect.width > 0 && rect.height > 0);
      }
      function releaseStaleConsentBackdrop(){
        if(!root.classList.contains("kdoc-fern-host") || !global.document){ return; }
        var backdrop = document.querySelector(".onetrust-pc-dark-filter");
        if(!backdrop){ return; }
        var dialog = document.getElementById("onetrust-pc-sdk");
        var banner = document.getElementById("onetrust-banner-sdk");
        if(!elementVisible(dialog) && !elementVisible(banner)){
          backdrop.style.pointerEvents = "none";
        }
      }
      function scheduleConsentBackdropRelease(){
        releaseStaleConsentBackdrop();
        if(!root.classList.contains("kdoc-fern-host") || !global.setTimeout){ return; }
        staleBackdropTimers.push(setTimeout(releaseStaleConsentBackdrop, 250));
        staleBackdropTimers.push(setTimeout(releaseStaleConsentBackdrop, 1000));
      }

      function focusPath(path, options){
        path = String(path || "").toLowerCase();
        if(!path){ return false; }
        for(var i = 0; i < fieldStates.length; i++){
          if(fieldStates[i].path === path){
            select(fieldStates[i].line, options || {scroll:true});
            return true;
          }
        }
        return false;
      }
      function setPathExpanded(path, value){
        path = String(path || "").toLowerCase();
        if(!path){ return false; }
        for(var i = 0; i < fieldStates.length; i++){
          if(fieldStates[i].path === path){
            setExpanded(fieldStates[i].line, value);
            applyFolds();
            scheduleCommentWrap();
            return true;
          }
        }
        return false;
      }

      root.addEventListener("click", handleRootClick, true);
      root.addEventListener("focusin", handleFocusIn);
      root.addEventListener("focusout", handleFocusOut);
      var keyTarget = scopedKeyboard ? root : document;
      keyTarget.addEventListener("keydown", handleCursorKey);
      if(wrapComments){
        wrapComments.addEventListener("change", handleWrapChange);
      }
      window.addEventListener("resize", handleResize);
      applyCommentWrap();
      applyFolds();
      scheduleConsentBackdropRelease();
      select(visibleFieldLines()[0] || lines[0]);

      controller = {
        root: root,
        destroy: function(){
          root.removeEventListener("click", handleRootClick, true);
          root.removeEventListener("focusin", handleFocusIn);
          root.removeEventListener("focusout", handleFocusOut);
          keyTarget.removeEventListener("keydown", handleCursorKey);
          if(wrapComments){ wrapComments.removeEventListener("change", handleWrapChange); }
          window.removeEventListener("resize", handleResize);
          staleBackdropTimers.forEach(function(timer){ clearTimeout(timer); });
          staleBackdropTimers = [];
          clearSelection();
          clearFilterHighlights();
          root.__kubectlDocController = null;
          root.classList.remove("kdoc-has-focus");
        },
        snapshot: function(){
          return {
            currentPath: currentLine ? currentLine.getAttribute("data-path") || "" : "",
            filter: filterQuery
          };
        },
        focusPath: focusPath,
        expandPath: function(path){ return setPathExpanded(path, true); },
        collapsePath: function(path){ return setPathExpanded(path, false); },
        setFilter: setFilter,
        clearFilter: clearFilter
      };
      root.__kubectlDocController = controller;
      return controller;
  }

  global.KubectlDoc = global.KubectlDoc || {};
  global.KubectlDoc.mount = mount;
  ready(function(){
    document.querySelectorAll("[data-kubectl-doc]").forEach(function(root){
      mount(root);
    });
  });
})(window);
