import os
import subprocess
import logging

def get_skill_info():
    return {
        "name": "system_update",
        "description": "Checks for source code updates from GitHub and performs a self-update/rebuild if a newer version is available.",
        "parameters": {
            "type": "object",
            "properties": {
                "force": {
                    "type": "boolean",
                    "description": "If true, skip version check and perform update immediately."
                }
            }
        }
    }

def execute(params):
    force = params.get("force", False)
    repo_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    
    logging.info(f"🚀 Starting system update check in {repo_dir}...")
    
    try:
        # 1. Check for remote updates (Git fetch)
        subprocess.run(["git", "-C", repo_dir, "fetch", "origin"], check=True)
        
        # 2. Compare local vs remote
        local_commit = subprocess.check_output(["git", "-C", repo_dir, "rev-parse", "HEAD"]).decode().strip()
        remote_commit = subprocess.check_output(["git", "-C", repo_dir, "rev-parse", "@{u}"]).decode().strip()
        
        if local_commit == remote_commit and not force:
            return {"status": "success", "message": "System is already up to date.", "current_version": local_commit[:7]}
        
        # 3. Perform update (Calls the existing installer script)
        logging.info("✨ New version detected! Triggering self-update script...")
        
        # Determine which update command to use based on environment
        update_cmd = "ottoclaw update"
        if os.path.exists("/data/data/com.termux"): # Termux environment
            update_cmd = "ottoclaw update"
        elif os.geteuid() != 0: # If not root, might need sudo
            update_cmd = "sudo ottoclaw update"
            
        # We run this in background and exit to let the service restart
        subprocess.Popen(["bash", "-c", f"sleep 2 && {update_cmd}"], start_new_session=True)
        
        return {
            "status": "updating",
            "message": "Update triggered successfully. The service will restart in 2 seconds.",
            "old_version": local_commit[:7],
            "new_version": remote_commit[:7]
        }
        
    except Exception as e:
        return {"status": "error", "message": f"Update failed: {str(e)}"}
